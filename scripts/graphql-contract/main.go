// graphql-contract generates reviewable API contracts from the broker's schema sources.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

type contract struct {
	Schema   *ast.Schema
	Sources  []*ast.Source
	Manifest []map[string]string
}

func readFull(root string) (*contract, error) {
	// Read the same ordered list the runtime loads. Fail closed if the loader is refactored.
	loader := filepath.Join(root, "broker/src/main/kotlin/graphql/GraphQLServer.kt")
	b, e := os.ReadFile(loader)
	if e != nil {
		return nil, e
	}
	block := regexp.MustCompile(`(?s)private fun loadSchema\(\): String \{\s*val schemaFiles = listOf\((.*?)\n\s*\)`).FindSubmatch(b)
	if len(block) != 2 {
		return nil, fmt.Errorf("cannot find runtime schema list in %s; update the contract loader with the runtime loader", loader)
	}
	matches := regexp.MustCompile(`"(schema-[a-z0-9-]+\.graphqls)"`).FindAllSubmatch(block[1], -1)
	paths := []string{}
	for _, m := range matches {
		paths = append(paths, filepath.Join(root, "broker/src/main/resources", string(m[1])))
	}
	return readSources(root, paths)
}
func readEdge(root string) (*contract, error) {
	b, e := os.ReadFile(filepath.Join(root, "gqlgen.yml"))
	if e != nil {
		return nil, e
	}
	if !strings.Contains(string(b), "schema:\n  - internal/graphql/schema/*.graphqls\n") {
		return nil, fmt.Errorf("Edge gqlgen schema inputs changed; update contract loader")
	}
	paths, e := filepath.Glob(filepath.Join(root, "internal/graphql/schema/*.graphqls"))
	if e != nil {
		return nil, e
	}
	return readSources(root, paths)
}
func readSources(root string, paths []string) (*contract, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no schema sources found")
	}
	c := &contract{}
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			return nil, fmt.Errorf("duplicate schema source %s", p)
		}
		seen[p] = true
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return nil, e
		}
		rel = filepath.ToSlash(rel)
		sum := sha256.Sum256(b)
		c.Sources = append(c.Sources, &ast.Source{Name: rel, Input: string(b)})
		c.Manifest = append(c.Manifest, map[string]string{"path": rel, "sha256": hex.EncodeToString(sum[:])})
	}
	var e error
	c.Schema, e = gqlparser.LoadSchema(c.Sources...)
	if e != nil {
		return nil, fmt.Errorf("invalid broker schema: %w", e)
	}
	return c, nil
}
func names[T any](m map[string]T) []string {
	out := []string{}
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
func description(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func defaultValue(v *ast.Value) any {
	if v == nil {
		return nil
	}
	return v.String()
}
func deprecated(ds ast.DirectiveList) (bool, any) {
	d := ds.ForName("deprecated")
	if d == nil {
		return false, nil
	}
	a := d.Arguments.ForName("reason")
	if a == nil {
		return true, "No longer supported"
	}
	return true, a.Value.Raw
}
func typeRef(s *ast.Schema, t *ast.Type) map[string]any {
	if t.NonNull {
		copy := *t
		copy.NonNull = false
		return map[string]any{"kind": "NON_NULL", "name": nil, "ofType": typeRef(s, &copy)}
	}
	if t.Elem != nil {
		return map[string]any{"kind": "LIST", "name": nil, "ofType": typeRef(s, t.Elem)}
	}
	return map[string]any{"kind": string(s.Types[t.NamedType].Kind), "name": t.NamedType, "ofType": nil}
}
func inputValue(s *ast.Schema, n, d string, t *ast.Type, v *ast.Value) map[string]any {
	return map[string]any{"name": n, "description": description(d), "type": typeRef(s, t), "defaultValue": defaultValue(v)}
}
func introspection(c *contract, queries, mutations map[string]bool) map[string]any {
	s := c.Schema
	types := []any{}
	seen := map[string]bool{}
	var walk func(string)
	walk = func(n string) {
		if seen[n] || strings.HasPrefix(n, "__") {
			return
		}
		seen[n] = true
		d := s.Types[n]
		if d == nil {
			return
		}
		item := map[string]any{"kind": string(d.Kind), "name": n, "description": description(d.Description), "fields": nil, "inputFields": nil, "enumValues": nil, "interfaces": nil, "possibleTypes": nil}
		fields, inputs, enums, interfaces, possible := []any{}, []any{}, []any{}, []any{}, []any{}
		sortedFields := append(ast.FieldList{}, d.Fields...)
		sort.Slice(sortedFields, func(i, j int) bool { return sortedFields[i].Name < sortedFields[j].Name })
		for _, f := range sortedFields {
			if strings.HasPrefix(f.Name, "__") {
				continue
			}
			if s.Query != nil && n == s.Query.Name && queries != nil && !queries[f.Name] {
				continue
			}
			if s.Mutation != nil && n == s.Mutation.Name && mutations != nil && !mutations[f.Name] {
				continue
			}
			walk(f.Type.Name())
			if d.Kind == ast.InputObject {
				inputs = append(inputs, inputValue(s, f.Name, f.Description, f.Type, f.DefaultValue))
				continue
			}
			args := []any{}
			sortedArgs := append(ast.ArgumentDefinitionList{}, f.Arguments...)
			sort.Slice(sortedArgs, func(i, j int) bool { return sortedArgs[i].Name < sortedArgs[j].Name })
			for _, a := range sortedArgs {
				args = append(args, inputValue(s, a.Name, a.Description, a.Type, a.DefaultValue))
				walk(a.Type.Name())
			}
			dep, reason := deprecated(f.Directives)
			fields = append(fields, map[string]any{"name": f.Name, "description": description(f.Description), "args": args, "type": typeRef(s, f.Type), "isDeprecated": dep, "deprecationReason": reason})
		}
		evs := append(ast.EnumValueList{}, d.EnumValues...)
		sort.Slice(evs, func(i, j int) bool { return evs[i].Name < evs[j].Name })
		for _, v := range evs {
			dep, reason := deprecated(v.Directives)
			enums = append(enums, map[string]any{"name": v.Name, "description": description(v.Description), "isDeprecated": dep, "deprecationReason": reason})
		}
		for _, n := range d.Interfaces {
			interfaces = append(interfaces, typeRef(s, ast.NamedType(n, nil)))
			walk(n)
		}
		if d.Kind == ast.Interface || d.Kind == ast.Union {
			for _, p := range s.PossibleTypes[n] {
				possible = append(possible, typeRef(s, ast.NamedType(p.Name, nil)))
				walk(p.Name)
			}
		}
		switch d.Kind {
		case ast.Object, ast.Interface:
			item["fields"] = fields
			item["interfaces"] = interfaces
		case ast.InputObject:
			item["inputFields"] = inputs
		case ast.Enum:
			item["enumValues"] = enums
		}
		if d.Kind == ast.Interface || d.Kind == ast.Union {
			item["possibleTypes"] = possible
		}
		types = append(types, item)
	}
	root := func(d *ast.Definition) any {
		if d == nil {
			return nil
		}
		walk(d.Name)
		return map[string]any{"name": d.Name}
	}
	query, mutation, subscription := root(s.Query), root(s.Mutation), any(nil)
	if queries == nil {
		subscription = root(s.Subscription)
		for _, n := range names(s.Types) {
			walk(n)
		}
	}
	sort.Slice(types, func(i, j int) bool {
		return types[i].(map[string]any)["name"].(string) < types[j].(map[string]any)["name"].(string)
	})
	return map[string]any{"data": map[string]any{"__schema": map[string]any{"queryType": query, "mutationType": mutation, "subscriptionType": subscription, "types": types}}}
}
func markdown(c *contract, title string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s GraphQL reference\n\nGenerated from the schema loaded by the broker. Do not edit this file.\nSee [the contract guide](%sREADME.md) for generation, verification and CLI usage.\n\n", title, "../")
	for _, n := range names(c.Schema.Types) {
		d := c.Schema.Types[n]
		if d.BuiltIn || strings.HasPrefix(n, "__") {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n`%s`\n\n", n, d.Kind)
		if text := docText(d.Description, d.BeforeDescriptionComment, d.AfterDescriptionComment); text != "" {
			fmt.Fprintf(&b, "%s\n\n", text)
		}
		if len(d.Fields) > 0 {
			fmt.Fprintln(&b, "| Field | Type | Default | Description |\n|---|---|---|---|")
			for _, f := range d.Fields {
				if strings.HasPrefix(f.Name, "__") {
					continue
				}
				def := ""
				if f.DefaultValue != nil {
					def = f.DefaultValue.String()
				}
				dep, reason := deprecated(f.Directives)
				text := docText(f.Description, f.BeforeDescriptionComment, f.AfterDescriptionComment)
				if dep {
					text += " Deprecated: " + fmt.Sprint(reason)
				}
				fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s |\n", f.Name, f.Type.String(), cell(def), cell(text))
			}
			fmt.Fprintln(&b)
			for _, f := range d.Fields {
				if len(f.Arguments) == 0 {
					continue
				}
				fmt.Fprintf(&b, "### %s.%s arguments\n\n| Argument | Type | Default | Description |\n|---|---|---|---|\n", n, f.Name)
				for _, a := range f.Arguments {
					def := ""
					if a.DefaultValue != nil {
						def = a.DefaultValue.String()
					}
					fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s |\n", a.Name, a.Type.String(), cell(def), cell(docText(a.Description, a.BeforeDescriptionComment, a.AfterDescriptionComment)))
				}
				fmt.Fprintln(&b)
			}
		}
		if len(d.EnumValues) > 0 {
			for _, v := range d.EnumValues {
				fmt.Fprintf(&b, "- `%s` %s\n", v.Name, cell(docText(v.Description, v.BeforeDescriptionComment, v.AfterDescriptionComment)))
			}
			fmt.Fprintln(&b)
		}
	}
	return []byte(b.String())
}
func docText(desc string, groups ...*ast.CommentGroup) string {
	var comments *ast.CommentGroup
	for _, group := range groups {
		if group != nil {
			comments = group
			break
		}
	}
	if desc != "" {
		return desc
	}
	if comments == nil {
		return ""
	}
	parts := []string{}
	for _, c := range comments.List {
		parts = append(parts, strings.TrimSpace(c.Text()))
	}
	return strings.Join(parts, " ")
}
func cell(s string) string { return strings.NewReplacer("|", "\\|", "\n", " ", "\r", "").Replace(s) }
func jsonBytes(v any) []byte {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		panic(e)
	}
	return append(b, '\n')
}
func outputs(c *contract, dir, title string) map[string][]byte {
	var s strings.Builder
	s.WriteString("# Generated API contract. Edit broker schema sources, then regenerate.\n")
	for _, src := range c.Sources {
		fmt.Fprintf(&s, "\n# Source: %s\n%s\n", src.Name, src.Input)
	}
	return map[string][]byte{filepath.Join(dir, "schema.graphql"): []byte(s.String()), filepath.Join(dir, "introspection.json"): jsonBytes(introspection(c, nil, nil)), filepath.Join(dir, "reference.md"): markdown(c, title), filepath.Join(dir, "sources.json"): jsonBytes(c.Manifest)}
}
func writeOutputs(files map[string][]byte, check bool) error {
	stale := []string{}
	for _, p := range names(files) {
		old, e := os.ReadFile(p)
		if e == nil && bytes.Equal(old, files[p]) {
			continue
		}
		if check {
			stale = append(stale, p)
			continue
		}
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return e
		}
		if e := os.WriteFile(p, files[p], 0644); e != nil {
			return e
		}
		fmt.Println("Generated", p)
	}
	if len(stale) > 0 {
		return fmt.Errorf("stale/missing generated contracts (regenerate and review):\n  %s", strings.Join(stale, "\n  "))
	}
	return nil
}
func cliRoots(root string) (map[string]bool, map[string]bool, error) {
	file, e := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "cli/devices.go"), nil, 0)
	if e != nil {
		return nil, nil, e
	}
	queries := map[string]bool{"getDevices": true, "broker": true, "opcuaServers": true, "opcuaNodeBrowse": true}
	mutations := map[string]bool{}
	found := false
	goast.Inspect(file, func(n goast.Node) bool {
		spec, ok := n.(*goast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "deviceAdapters" {
			return true
		}
		found = true
		for _, value := range spec.Values {
			list, ok := value.(*goast.CompositeLit)
			if !ok {
				continue
			}
			for _, el := range list.Elts {
				row, ok := el.(*goast.CompositeLit)
				if !ok || len(row.Elts) != 4 {
					continue
				}
				vals := []string{}
				for _, el := range row.Elts {
					lit, ok := el.(*goast.BasicLit)
					if !ok {
						continue
					}
					v, e := strconv.Unquote(lit.Value)
					if e == nil {
						vals = append(vals, v)
					}
				}
				if len(vals) == 4 {
					mutations[vals[1]] = true
					queries[vals[2]] = true
				}
			}
		}
		return false
	})
	if !found || len(mutations) == 0 {
		return nil, nil, fmt.Errorf("could not read CLI device adapter registry")
	}
	return queries, mutations, nil
}
func main() {
	mainRoot := flag.String("main-root", "../../../main", "Full Broker checkout")
	edgeRoot := flag.String("edge-root", "", "Optional Edge checkout; also generate/check Edge reference")
	cliRoot := flag.String("cli-root", "", "Optional tools checkout; validate CLI operations and adapter registry")
	outputDir := flag.String("output-dir", "", "Artifact directory containing main/ and edge/ (default: main-root/doc/graphql)")
	check := flag.Bool("check", false, "Fail on drift without changing generated files")
	liveURL := flag.String("live-url", "", "Compare read-only live introspection to Full source (use --live-edge for Edge)")
	liveEdge := flag.Bool("live-edge", false, "Compare live broker against --edge-root schema")
	flag.Parse()
	if flag.NArg() != 0 {
		fail(fmt.Errorf("unexpected arguments: %v", flag.Args()))
	}
	full, e := readFull(*mainRoot)
	if e != nil {
		fail(e)
	}
	if *outputDir == "" {
		*outputDir = filepath.Join(*mainRoot, "doc/graphql")
	}
	files := outputs(full, filepath.Join(*outputDir, "main"), "Full Broker")
	var edge *contract
	if *edgeRoot != "" {
		edge, e = readEdge(*edgeRoot)
		if e != nil {
			fail(e)
		}
		for p, b := range outputs(edge, filepath.Join(*outputDir, "edge"), "Edge") {
			files[p] = b
		}
	}
	if *cliRoot != "" {
		if e := checkCLIOperations(*cliRoot, full); e != nil {
			fail(e)
		}
		_, m, e := cliRoots(*cliRoot)
		if e != nil {
			fail(e)
		}
		for root := range m {
			if full.Schema.Mutation.Fields.ForName(root) == nil {
				fail(fmt.Errorf("CLI adapter API %s missing from Full Broker", root))
			}
		}

	}
	if *liveURL != "" {
		expected := full
		if *liveEdge {
			if edge == nil {
				fail(fmt.Errorf("--live-edge requires --edge-root"))
			}
			expected = edge
		}
		if e := checkLive(*liveURL, expected); e != nil {
			fail(e)
		}
	}
	if e := writeOutputs(files, *check); e != nil {
		fail(e)
	}
	if *check {
		fmt.Println("GraphQL contracts are current")
	}
}
func fail(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
