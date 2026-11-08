package main

// Generic, schema-driven access to every GraphQL operation of the connected broker.
//
//	mmq gql <query|@file|-> [--vars json|@file|-]   raw GraphQL
//	mmq schema [filter|Type]                         introspect operations and types
//	mmq api <path> [--args json|@file|-] [k=v ...]   call any query/mutation by path
//	mmq api template|describe <path>                 skeleton arguments / signature
//	mmq <noun> <verb> ...                            friendly aliases (user, namespace, ...)
//
// Operations are built from live introspection, so the CLI follows the broker (Full or Edge).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ---------- input helpers ----------

// readJSONSource reads inline JSON, @file, a file path ending in .json, or "-" (stdin).
func readJSONSource(src string) ([]byte, error) {
	switch {
	case src == "-":
		return io.ReadAll(os.Stdin)
	case strings.HasPrefix(src, "@"):
		return os.ReadFile(src[1:])
	case strings.HasPrefix(strings.TrimSpace(src), "{") || strings.HasPrefix(strings.TrimSpace(src), "["):
		return []byte(src), nil
	default:
		return os.ReadFile(src)
	}
}

func parseJSONObject(src string) (map[string]any, error) {
	b, err := readJSONSource(src)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	if obj == nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	return obj, nil
}

// ---------- introspection helpers ----------

type opInfo struct {
	Path        string    `json:"path"`
	Kind        string    `json:"kind"` // query | mutation
	Description string    `json:"description,omitempty"`
	Args        []argInfo `json:"args"`
	Returns     string    `json:"returns"`
}

type argInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

func toArgInfos(fields []gqlField) []argInfo {
	out := []argInfo{}
	for _, f := range fields {
		out = append(out, argInfo{
			Name: f.Name, Type: f.Type.String(),
			Required:    f.Type.Kind == "NON_NULL" && f.DefaultValue == nil,
			Default:     deref(f.DefaultValue),
			Description: strings.TrimSpace(f.Description),
		})
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// isNamespaceField: a no-arg field whose object type only groups operations (e.g. mutation { user { ... } }).
func (s *deviceSchema) isNamespaceField(f gqlField) bool {
	if len(f.Args) > 0 || f.Type.base() == "" {
		return false
	}
	t := s.Types[f.Type.base()]
	if t.Kind != "OBJECT" {
		return false
	}
	n := t.Name
	return strings.HasSuffix(n, "Mutations") || strings.HasSuffix(n, "Mutation") ||
		strings.HasSuffix(n, "Query") || strings.HasSuffix(n, "Queries")
}

func (s *deviceSchema) operations() []opInfo {
	ops := []opInfo{}
	add := func(kind, rootType string) {
		root := s.Types[rootType]
		for _, f := range root.Fields {
			if strings.HasPrefix(f.Name, "__") {
				continue
			}
			if s.isNamespaceField(f) {
				for _, sub := range s.Types[f.Type.base()].Fields {
					ops = append(ops, opInfo{Path: f.Name + "." + sub.Name, Kind: kind, Description: firstLine(sub.Description), Args: toArgInfos(sub.Args), Returns: sub.Type.String()})
				}
				continue
			}
			ops = append(ops, opInfo{Path: f.Name, Kind: kind, Description: firstLine(f.Description), Args: toArgInfos(f.Args), Returns: f.Type.String()})
		}
	}
	add("query", s.Query)
	if s.Mutation != "" {
		add("mutation", s.Mutation)
	}
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Kind != ops[j].Kind {
			return ops[i].Kind == "query"
		}
		return ops[i].Path < ops[j].Path
	})
	return ops
}

// resolve finds an operation by path ("users", "user.createUser", "mutation:publish", "query:session").
func (s *deviceSchema) resolve(path string) (kind string, segs []gqlField, err error) {
	want := ""
	if i := strings.IndexByte(path, ':'); i > 0 {
		want, path = path[:i], path[i+1:]
		if want != "query" && want != "mutation" {
			return "", nil, fmt.Errorf("unknown operation prefix %q (use query: or mutation:)", want)
		}
	}
	parts := strings.Split(path, ".")
	if len(parts) < 1 || len(parts) > 2 || parts[0] == "" {
		return "", nil, fmt.Errorf("invalid operation path %q (use name or group.name)", path)
	}
	try := func(k, root string) []gqlField {
		if root == "" || (want != "" && want != k) {
			return nil
		}
		f0, ok := fieldNamed(s.Types[root].Fields, parts[0])
		if !ok {
			return nil
		}
		if len(parts) == 1 {
			return []gqlField{f0}
		}
		f1, ok := fieldNamed(s.Types[f0.Type.base()].Fields, parts[1])
		if !ok {
			return nil
		}
		return []gqlField{f0, f1}
	}
	if r := try("query", s.Query); r != nil {
		return "query", r, nil
	}
	if r := try("mutation", s.Mutation); r != nil {
		return "mutation", r, nil
	}
	return "", nil, fmt.Errorf("operation %q is not available on this broker (see: mmq schema)", path)
}

// selection builds a result selection from scalars/enums plus nested objects up to depth.
func (s *deviceSchema) selection(ref gqlRef, depth int, seen map[string]bool) string {
	t := s.Types[ref.base()]
	if t.Kind != "OBJECT" && t.Kind != "INTERFACE" {
		return ""
	}
	if seen[t.Name] {
		return ""
	}
	seen[t.Name] = true
	defer delete(seen, t.Name)
	parts := []string{}
	for _, f := range t.Fields {
		required := false
		for _, a := range f.Args {
			if a.Type.Kind == "NON_NULL" && a.DefaultValue == nil {
				required = true
			}
		}
		if required || strings.HasPrefix(f.Name, "__") {
			continue
		}
		ft := s.Types[f.Type.base()]
		switch ft.Kind {
		case "OBJECT", "INTERFACE":
			if depth <= 1 {
				continue
			}
			if sub := s.selection(f.Type, depth-1, seen); sub != "" {
				parts = append(parts, f.Name+" { "+sub+" }")
			}
		case "UNION":
			continue
		default:
			parts = append(parts, f.Name)
		}
	}
	return strings.Join(parts, " ")
}

// ---------- argument coercion ----------

// coerceKV converts "key=value" text using the argument type (strings stay strings).
func coerceKV(raw string, ref gqlRef) any {
	base := ref.base()
	// key=@file reads the value from a file (e.g. document=@mapping.json) when the file exists.
	if strings.HasPrefix(raw, "@") && len(raw) > 1 {
		if b, err := os.ReadFile(raw[1:]); err == nil {
			if base == "String" || base == "ID" {
				return string(b)
			}
			var v any
			if json.Unmarshal(b, &v) == nil {
				return v
			}
			return string(b)
		}
	}
	if ref.Kind != "LIST" && (base == "String" || base == "ID") {
		return raw
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	return raw
}

// shapeArgs wraps flat fields into the single input-object argument when that is the only argument.
func (s *deviceSchema) shapeArgs(f gqlField, in map[string]any) map[string]any {
	if len(f.Args) != 1 {
		return in
	}
	only := f.Args[0]
	it := s.Types[only.Type.base()]
	if it.Kind != "INPUT_OBJECT" || only.Type.Kind == "LIST" || (only.Type.OfType != nil && only.Type.OfType.Kind == "LIST") {
		return in
	}
	if _, has := in[only.Name]; has || len(in) == 0 {
		return in
	}
	for k := range in {
		if _, ok := fieldNamed(it.InputFields, k); !ok {
			return in
		}
	}
	return map[string]any{only.Name: in}
}

// ---------- operation execution ----------

type apiOptions struct {
	ArgsSrc  string
	KV       []string
	Select   string
	Depth    int
	DryRun   bool
	Reveal   bool
	Operands []string
}

func parseAPIOptions(args []string) (apiOptions, error) {
	o := apiOptions{Depth: 3}
	for i := 0; i < len(args); i++ {
		a := args[i]
		need := func() (string, error) {
			i++
			if i >= len(args) {
				return "", fmt.Errorf("%s requires a value", a)
			}
			return args[i], nil
		}
		var err error
		switch a {
		case "--args", "--vars", "--input", "--file":
			o.ArgsSrc, err = need()
		case "--select":
			o.Select, err = need()
		case "--depth":
			var v string
			if v, err = need(); err == nil {
				if _, e := fmt.Sscanf(v, "%d", &o.Depth); e != nil || o.Depth < 1 {
					err = fmt.Errorf("--depth must be a positive integer")
				}
			}
		case "--dry-run":
			o.DryRun = true
		case "--reveal", "--no-redact":
			o.Reveal = true
		case "--arg":
			var v string
			if v, err = need(); err == nil {
				o.KV = append(o.KV, v)
			}
		case "-":
			if o.ArgsSrc == "" {
				o.ArgsSrc = "-"
			} else {
				o.Operands = append(o.Operands, a)
			}
		default:
			if strings.HasPrefix(a, "--") {
				return o, fmt.Errorf("unknown option %s", a)
			}
			if strings.HasPrefix(a, "@") && o.ArgsSrc == "" {
				o.ArgsSrc = a
			} else if i := strings.IndexByte(a, '='); i > 0 && !strings.ContainsAny(a[:i], " {}/") {
				o.KV = append(o.KV, a)
			} else if strings.HasSuffix(a, ".json") && o.ArgsSrc == "" {
				o.ArgsSrc = a
			} else {
				o.Operands = append(o.Operands, a)
			}
		}
		if err != nil {
			return o, err
		}
	}
	return o, nil
}

func (o apiOptions) collect(s *deviceSchema, f gqlField) (map[string]any, error) {
	in := map[string]any{}
	if o.ArgsSrc != "" {
		b, err := readJSONSource(o.ArgsSrc)
		if err != nil {
			return nil, err
		}
		if t := strings.TrimSpace(string(b)); strings.HasPrefix(t, "[") && len(f.Args) == 1 {
			var arr []any
			if err := json.Unmarshal(b, &arr); err != nil {
				return nil, fmt.Errorf("invalid JSON array: %w", err)
			}
			in = map[string]any{f.Args[0].Name: arr}
		} else if err := json.Unmarshal(b, &in); err != nil || in == nil {
			return nil, fmt.Errorf("invalid JSON object for arguments")
		}
	}
	for _, kv := range o.KV {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			return nil, fmt.Errorf("expected key=value, got %q", kv)
		}
		k, raw := kv[:i], kv[i+1:]
		ref := gqlRef{Kind: "SCALAR", Name: "JSON"}
		if a, ok := fieldNamed(f.Args, k); ok {
			ref = a.Type
		} else if len(f.Args) == 1 {
			if inf, ok := fieldNamed(s.Types[f.Args[0].Type.base()].InputFields, k); ok {
				ref = inf.Type
			}
		}
		in[k] = coerceKV(raw, ref)
	}
	if hasRedactedValue(in) {
		return nil, fmt.Errorf("input contains [REDACTED]; omit these fields instead")
	}
	return s.shapeArgs(f, in), nil
}

// buildOperation returns the GraphQL document and variables for a path.
func (s *deviceSchema) buildOperation(path string, o apiOptions) (doc string, vars map[string]any, kind string, segs []gqlField, err error) {
	kind, segs, err = s.resolve(path)
	if err != nil {
		return
	}
	leaf := segs[len(segs)-1]
	if len(segs) == 2 {
		for _, a := range segs[0].Args {
			if a.Type.Kind == "NON_NULL" {
				err = fmt.Errorf("%s has required arguments and cannot be used as a group", segs[0].Name)
				return
			}
		}
	}
	vars, err = o.collect(s, leaf)
	if err != nil {
		return
	}
	for k := range vars {
		if _, ok := fieldNamed(leaf.Args, k); !ok {
			names := []string{}
			for _, a := range leaf.Args {
				names = append(names, a.Name)
			}
			err = fmt.Errorf("%s: unknown argument %q (arguments: %s)", path, k, strings.Join(names, ", "))
			return
		}
	}
	defs, uses := []string{}, []string{}
	for _, a := range leaf.Args {
		v, ok := vars[a.Name]
		if !ok && a.DefaultValue != nil {
			continue
		}
		if err = s.validate(a.Type, v, a.Name); err != nil {
			return
		}
		if ok {
			defs = append(defs, "$"+a.Name+": "+a.Type.String())
			uses = append(uses, a.Name+": $"+a.Name)
		}
	}
	sel := strings.TrimSpace(o.Select)
	if strings.HasPrefix(sel, "{") && strings.HasSuffix(sel, "}") {
		sel = strings.TrimSpace(sel[1 : len(sel)-1])
	}
	if sel == "" {
		sel = s.selection(leaf.Type, o.Depth, map[string]bool{})
	}
	if s.Types[leaf.Type.base()].Kind == "OBJECT" && sel == "" {
		sel = "__typename"
	}
	call := leaf.Name
	if len(uses) > 0 {
		call += "(" + strings.Join(uses, ", ") + ")"
	}
	if sel != "" && s.Types[leaf.Type.base()].Kind != "SCALAR" && s.Types[leaf.Type.base()].Kind != "ENUM" {
		call += " { " + sel + " }"
	}
	if len(segs) == 2 {
		call = segs[0].Name + " { " + call + " }"
	}
	head := kind
	if len(defs) > 0 {
		head += " ApiCall(" + strings.Join(defs, ", ") + ")"
	}
	doc = head + " { " + call + " }"
	return
}

func resultOf(data map[string]any, segs []gqlField) any {
	var cur any = data
	for _, f := range segs {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[f.Name]
	}
	return cur
}

// failed reports mutation-style failures ({success:false}, non-empty errors).
func failedResult(v any) bool {
	switch x := v.(type) {
	case bool:
		return !x
	case map[string]any:
		if s, ok := x["success"].(bool); ok && !s {
			return true
		}
		if es, ok := x["errors"].([]any); ok && len(es) > 0 {
			return true
		}
	case []any:
		for _, item := range x {
			if m, ok := item.(map[string]any); ok {
				if s, ok := m["success"].(bool); ok && !s {
					return true
				}
			}
		}
	}
	return false
}

func executeAPI(ctx context.Context, c *Client, path string, o apiOptions) error {
	s, err := loadDeviceSchema(ctx, c)
	if err != nil {
		return err
	}
	doc, vars, kind, segs, err := s.buildOperation(path, o)
	if err != nil {
		return err
	}
	if o.DryRun {
		return printJSON(map[string]any{"dryRun": true, "kind": kind, "query": doc, "variables": vars})
	}
	data, err := deviceQuery(ctx, c, doc, vars)
	if err != nil {
		return err
	}
	res := resultOf(data, segs)
	if !o.Reveal {
		res = redactDevice(res)
	}
	if err := printJSON(res); err != nil {
		return err
	}
	if kind == "mutation" && failedResult(res) {
		return fmt.Errorf("%s reported failure", path)
	}
	return nil
}

// ---------- commands ----------

const apiHelp = `Usage: mmq api <path> [--args json|@file|-] [key=value ...] [options]
       mmq api describe <path>      Signature, argument types and descriptions
       mmq api template <path>      Argument skeleton (required fields) as JSON

Calls any GraphQL query or mutation of the connected broker. <path> is "name" or
"group.name" (e.g. users, user.createUser, publish, topicNamespace.create).
Prefix with query: or mutation: when a name exists in both (session, hmi, ...).

Arguments:
  --args <json|@file|->  Arguments as JSON object (inline, file, or stdin)
  key=value              Single argument; JSON parsed unless the type is String.
                         If the operation has exactly one input object, its fields may be given flat.
Options:
  --select "a b { c }"   Override the result selection set
  --depth N              Nesting depth of the generated selection (default 3)
  --dry-run              Print the generated GraphQL document and variables only
  --reveal               Do not redact password/secret/token fields in output
Output is JSON. Non-zero exit on GraphQL errors or {success:false}.
List operations: mmq schema
`

func runAPI(ctx context.Context, c *Client, args []string) error {
	if len(args) == 0 || hasHelpFlag(args[:min(1, len(args))]) {
		fmt.Print(apiHelp)
		return nil
	}
	switch args[0] {
	case "describe", "template":
		if len(args) < 2 {
			return fmt.Errorf("usage: mmq api %s <path>", args[0])
		}
		if args[0] == "describe" {
			return runSchema(ctx, c, []string{args[1]})
		}
		return runAPITemplate(ctx, c, args[1])
	case "list":
		return runSchema(ctx, c, args[1:])
	}
	o, err := parseAPIOptions(args[1:])
	if err != nil {
		return err
	}
	if len(o.Operands) > 0 {
		return fmt.Errorf("unexpected arguments %v (use key=value or --args)", o.Operands)
	}
	return executeAPI(ctx, c, args[0], o)
}

func runAPITemplate(ctx context.Context, c *Client, path string) error {
	s, err := loadDeviceSchema(ctx, c)
	if err != nil {
		return err
	}
	_, segs, err := s.resolve(path)
	if err != nil {
		return err
	}
	leaf := segs[len(segs)-1]
	out := map[string]any{}
	for _, a := range leaf.Args {
		if a.Type.Kind == "NON_NULL" && a.DefaultValue == nil {
			out[a.Name] = s.template(a.Type, 0)
		}
	}
	return printJSON(out)
}

const gqlHelp = `Usage: mmq gql <query|@file|-> [--vars json|@file|-] [--reveal]

Send a raw GraphQL document to the broker and print the "data" object as JSON.
Exit code is non-zero if the response contains errors.
Use "mmq schema" to discover operations and "mmq schema <Type>" for input types.
Example: mmq gql '{ brokers { nodeId } }'
`

func runGQL(ctx context.Context, c *Client, args []string) error {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Print(gqlHelp)
		return nil
	}
	var query, varsSrc string
	reveal := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--vars", "--variables", "--args":
			i++
			if i >= len(args) {
				return fmt.Errorf("%s requires a value", args[i-1])
			}
			varsSrc = args[i]
		case "--reveal", "--no-redact":
			reveal = true
		default:
			if query != "" {
				return fmt.Errorf("unexpected argument %q", args[i])
			}
			query = args[i]
		}
	}
	if query == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		query = string(b)
	} else if strings.HasPrefix(query, "@") {
		b, err := os.ReadFile(query[1:])
		if err != nil {
			return err
		}
		query = string(b)
	}
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("empty GraphQL document")
	}
	var vars map[string]any
	if varsSrc != "" {
		var err error
		if vars, err = parseJSONObject(varsSrc); err != nil {
			return err
		}
	}
	data, err := deviceQuery(ctx, c, query, vars)
	if err != nil {
		return err
	}
	var out any = data
	if !reveal {
		out = redactDevice(data)
	}
	return printJSON(out)
}

const schemaHelp = `Usage: mmq schema [filter|Type|path] [--queries|--mutations]

Introspect the connected broker (Full or Edge).
  mmq schema                 List every query and mutation (path, arguments, return type)
  mmq schema user            Operations whose path contains "user"
  mmq schema user.createUser Full signature with argument and input-type details
  mmq schema CreateUserInput Describe a type (fields, enum values)
Use --json for structured output.
`

func runSchema(ctx context.Context, c *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Print(schemaHelp)
		return nil
	}
	var filter string
	onlyKind := ""
	for _, a := range args {
		switch a {
		case "--queries":
			onlyKind = "query"
		case "--mutations":
			onlyKind = "mutation"
		default:
			if strings.HasPrefix(a, "--") {
				return fmt.Errorf("unknown option %s", a)
			}
			filter = a
		}
	}
	s, err := loadDeviceSchema(ctx, c)
	if err != nil {
		return err
	}
	if filter != "" {
		if t, ok := s.Types[filter]; ok && !strings.HasPrefix(filter, "__") {
			return printJSON(describeType(t))
		}
		if kind, segs, err := s.resolve(filter); err == nil {
			return printJSON(s.describeOp(kind, filter, segs))
		}
	}
	ops := []opInfo{}
	for _, op := range s.operations() {
		if onlyKind != "" && op.Kind != onlyKind {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(op.Path), strings.ToLower(filter)) {
			continue
		}
		ops = append(ops, op)
	}
	if filter != "" && len(ops) == 0 {
		return fmt.Errorf("nothing matches %q (try: mmq schema)", filter)
	}
	if c.cfg.JSONMode {
		return printJSON(ops)
	}
	for _, op := range ops {
		sig := make([]string, 0, len(op.Args))
		for _, a := range op.Args {
			sig = append(sig, a.Name+": "+a.Type)
		}
		line := fmt.Sprintf("%-8s %s(%s) -> %s", op.Kind, op.Path, strings.Join(sig, ", "), op.Returns)
		if op.Description != "" {
			line += "  # " + op.Description
		}
		fmt.Println(line)
	}
	return nil
}

func describeType(t gqlType) map[string]any {
	m := map[string]any{"name": t.Name, "kind": t.Kind}
	if d := strings.TrimSpace(t.Description); d != "" {
		m["description"] = d
	}
	fields := t.InputFields
	key := "inputFields"
	if t.Kind != "INPUT_OBJECT" {
		fields = t.Fields
		key = "fields"
	}
	if len(fields) > 0 {
		list := []map[string]any{}
		for _, f := range fields {
			e := map[string]any{"name": f.Name, "type": f.Type.String(), "required": f.Type.Kind == "NON_NULL" && f.DefaultValue == nil}
			if f.DefaultValue != nil {
				e["default"] = *f.DefaultValue
			}
			if d := strings.TrimSpace(f.Description); d != "" {
				e["description"] = d
			}
			if len(f.Args) > 0 {
				e["args"] = toArgInfos(f.Args)
			}
			list = append(list, e)
		}
		m[key] = list
	}
	if len(t.EnumValues) > 0 {
		vals := []string{}
		for _, v := range t.EnumValues {
			vals = append(vals, v.Name)
		}
		m["enumValues"] = vals
	}
	return m
}

func (s *deviceSchema) describeOp(kind, path string, segs []gqlField) map[string]any {
	leaf := segs[len(segs)-1]
	types := map[string]gqlType{}
	for _, a := range leaf.Args {
		s.inputTypes(a.Type, types)
	}
	names := make([]string, 0, len(types))
	for n := range types {
		names = append(names, n)
	}
	sort.Strings(names)
	td := []map[string]any{}
	for _, n := range names {
		td = append(td, describeType(types[n]))
	}
	return map[string]any{
		"path": path, "kind": kind, "description": strings.TrimSpace(leaf.Description),
		"args": toArgInfos(leaf.Args), "returns": leaf.Type.String(), "inputTypes": td,
	}
}

// ---------- noun commands (friendly aliases over the generic engine) ----------

type nounSpec struct {
	Names    []string          // accepted command names
	Mutation string            // mutation group ("" = none)
	Mutations map[string]string // verb -> top-level (ungrouped) mutation field
	QueryGroup string          // query group whose fields are verbs ("" = none)
	Brokers  string            // "" = full, "edge", "both"
	Queries  map[string]string // verb -> query field
	Help     string
}

var nounSpecs = []nounSpec{
	{Names: []string{"user", "users"}, Brokers: "both", Mutation: "user", Queries: map[string]string{"list": "users"}, Help: "Manage broker users (admin): list, createUser, updateUser, deleteUser, setPassword, ..."},
	{Names: []string{"archiveGroup"}, Brokers: "both", Mutation: "archiveGroup", Queries: map[string]string{"list": "archiveGroups", "get": "archiveGroup"}, Help: "Manage archive groups: list, get, create, update, delete, enable, disable"},
	{Names: []string{"topicSchema", "schemaPolicy"}, Mutation: "topicSchemaPolicy", Queries: map[string]string{"list": "topicSchemaPolicies", "get": "topicSchemaPolicy", "validate": "topicSchemaValidate"}, Help: "Payload schema policies: list, get, validate, create, update, delete"},
	{Names: []string{"namespace", "topicNamespace"}, Mutation: "topicNamespace", Queries: map[string]string{"list": "topicNamespaces", "get": "topicNamespace", "match": "topicNamespaceMatch"}, Help: "Topic namespaces: list, get, match, create, update, delete"},
	{Names: []string{"flow"}, Mutation: "flow", Queries: map[string]string{"classes": "flowClasses", "instances": "flowInstances", "nodeTypes": "flowNodeTypes", "list": "flowInstances"}, Help: "Flow engine: classes, instances, nodeTypes + flow mutations"},
	{Names: []string{"kafka"}, Brokers: "both", Queries: map[string]string{"consumerGroups": "kafkaConsumerGroups", "messages": "kafkaMessages", "offsets": "kafkaTopicOffsets"}, Help: "Kafka server inspection: consumerGroups, messages, offsets"},
	{Names: []string{"broker", "brokers", "nodes"}, Brokers: "both", Queries: map[string]string{"list": "brokers", "get": "broker"}, Help: "Cluster nodes: list, get"},
	{Names: []string{"agent", "agents"}, Mutation: "agent", Queries: map[string]string{"list": "agents", "get": "agent"}, Help: "AI agents: list, get, create, update, delete, ..."},
	{Names: []string{"mcpServer", "mcp"}, Mutation: "mcpServer", Queries: map[string]string{"list": "mcpServers", "get": "mcpServer"}, Help: "MCP servers: list, get, create, update, delete, ..."},
	{Names: []string{"uns", "unsModeler"}, Mutation: "unsModeler", Brokers: "edge", Queries: map[string]string{"list": "unsModelers", "validate": "unsModelerValidate", "schema": "unsModelerSchema", "prompt": "unsModelerDefaultPrompt"}, Help: "WinCC OA UNS Modeler (Edge): list, schema (mapping JSON Schema), prompt, validate, create, update, delete, toggle, saveDraft, apply, createInWinCCOa, importProposal, startAiRun, cancelAiRun"},
	{Names: []string{"dataCatalog"}, Mutation: "dataCatalog", Queries: map[string]string{"types": "dataCatalogTypes", "type": "dataCatalogType", "instances": "dataCatalogInstances", "instance": "dataCatalogInstance", "relations": "dataCatalogRelations", "infer": "inferDataCatalog"}, Help: "Data catalog (Core): types, type, instances, instance, relations, infer + saveType, deleteType, saveInstance, deleteInstance, saveRelation, deleteRelation, exportCatalog, importCatalog"},
	{Names: []string{"genai", "ai"}, QueryGroup: "genai", Brokers: "both", Queries: map[string]string{"providers": "configuredProviders"}, Help: "GenAI helpers: providers (configured), generate, analyzeTopics, proposeDataCatalog (see: mmq schema genai)"},
	{Names: []string{"rtspCamera", "camera"}, Mutation: "rtspCamera", Brokers: "edge", Queries: map[string]string{"list": "rtspCameras", "get": "rtspCamera"}, Help: "RTSP cameras (Edge): list, get, create, update, delete, start, stop, toggle, triggerSnapshot"},
	{Names: []string{"redfish"}, Brokers: "both", Queries: map[string]string{"list": "redfishMappings", "get": "redfishMapping", "sensors": "redfishLiveSensors"}, Mutations: map[string]string{"save": "saveRedfishMapping", "delete": "deleteRedfishMapping", "toggle": "toggleRedfishMapping"}, Help: "Redfish mappings: list, get, sensors, save, delete, toggle"},
	{Names: []string{"virtualTopicTree", "vtt"}, Brokers: "both", Queries: map[string]string{"list": "virtualTopicTrees", "get": "virtualTopicTree", "validate": "validateVirtualTopicTree", "preview": "previewVirtualTopicTree"}, Mutations: map[string]string{"save": "saveVirtualTopicTree", "delete": "deleteVirtualTopicTree"}, Help: "Virtual topic trees: list, get, validate, preview, save, delete (optimistic locking via expectedRevision)"},
	{Names: []string{"peerLink"}, Brokers: "both", Queries: map[string]string{"status": "peerLink", "list": "peerLink"}, Help: "PeerLink configuration and the state of every peer link on this node: status"},
	{Names: []string{"winccoa"}, QueryGroup: "winccoa", Brokers: "edge", Help: "WinCC OA browsing (Edge inside WCCOAmmq): verbs are the fields of the winccoa query group, see: mmq schema winccoa"},
	{Names: []string{"genAiProvider", "provider"}, Brokers: "both", Mutation: "genAiProvider", Queries: map[string]string{"list": "genAiProviders", "get": "genAiProvider"}, Help: "GenAI providers: list, get, create, update, delete, ..."},
}

func nounFor(cmd string) (nounSpec, bool) {
	for _, n := range nounSpecs {
		for _, name := range n.Names {
			if strings.EqualFold(name, cmd) {
				return n, true
			}
		}
	}
	return nounSpec{}, false
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (n nounSpec) usage() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: mmq %s <verb> [--args json|@file|-] [key=value ...] [--dry-run] [--select ...]\n\n%s\n\n", n.Names[0], n.Help)
	verbs := []string{}
	for v, q := range n.Queries {
		verbs = append(verbs, fmt.Sprintf("  %-16s query    %s", v, q))
	}
	for v, m := range n.Mutations {
		verbs = append(verbs, fmt.Sprintf("  %-16s mutation %s", v, m))
	}
	sort.Strings(verbs)
	b.WriteString("Verbs:\n" + strings.Join(verbs, "\n"))
	if n.Mutation != "" {
		fmt.Fprintf(&b, "\n  <name>           mutation %s.<name> (a prefix is allowed: create -> create%s)\n", n.Mutation, capitalize(n.Mutation))
	}
	b.WriteString("\nDiscover exact arguments: mmq schema " + firstNonEmpty(n.Mutation, n.QueryGroup) + "   |   mmq api describe <path>\n")
	return b.String()
}

func runNoun(ctx context.Context, c *Client, n nounSpec, args []string) error {
	if len(args) == 0 || hasHelpFlag(args[:1]) {
		fmt.Print(n.usage())
		return nil
	}
	verb, rest := args[0], args[1:]
	o, err := parseAPIOptions(rest)
	if err != nil {
		return err
	}
	// Positional operand => first required argument of the operation (e.g. `namespace get myNs`).
	path := ""
	if q, ok := n.Queries[verb]; ok {
		path = "query:" + q
	} else if m, ok := n.Mutations[verb]; ok {
		path = "mutation:" + m
	} else if n.QueryGroup != "" {
		path = "query:" + n.QueryGroup + "." + verb
	} else if n.Mutation != "" {
		s, err := loadDeviceSchema(ctx, c)
		if err != nil {
			return err
		}
		group, ok := fieldNamed(s.Types[s.Mutation].Fields, n.Mutation)
		if !ok {
			return fmt.Errorf("%s is not available on this broker", n.Names[0])
		}
		fields := s.Types[group.Type.base()].Fields
		for _, cand := range []string{verb, verb + capitalize(n.Mutation), verb + capitalize(n.Names[0])} {
			if _, ok := fieldNamed(fields, cand); ok {
				path = "mutation:" + n.Mutation + "." + cand
				break
			}
		}
		if path == "" {
			names := []string{}
			for _, f := range fields {
				names = append(names, f.Name)
			}
			return fmt.Errorf("unknown verb %q for %s (mutations: %s)", verb, n.Names[0], strings.Join(names, ", "))
		}
	} else {
		return fmt.Errorf("unknown verb %q for %s", verb, n.Names[0])
	}
	if len(o.Operands) > 0 {
		s, err := loadDeviceSchema(ctx, c)
		if err != nil {
			return err
		}
		_, segs, err := s.resolve(path)
		if err != nil {
			return err
		}
		leaf := segs[len(segs)-1]
		if len(o.Operands) > len(leaf.Args) {
			return fmt.Errorf("unexpected arguments %v", o.Operands)
		}
		for i, v := range o.Operands {
			o.KV = append(o.KV, leaf.Args[i].Name+"="+v)
		}
		o.Operands = nil
	}
	return executeAPI(ctx, c, path, o)
}
