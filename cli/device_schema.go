package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Device operations use the target's schema, not a frozen copy of one broker's inputs.
type gqlRef struct {
	Kind   string  `json:"kind"`
	Name   string  `json:"name"`
	OfType *gqlRef `json:"ofType"`
}

func (r gqlRef) base() string {
	if r.OfType != nil {
		return r.OfType.base()
	}
	return r.Name
}
func (r gqlRef) String() string {
	if r.Kind == "NON_NULL" {
		return r.OfType.String() + "!"
	}
	if r.Kind == "LIST" {
		return "[" + r.OfType.String() + "]"
	}
	return r.Name
}

type gqlField struct {
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Type         gqlRef     `json:"type"`
	DefaultValue *string    `json:"defaultValue"`
	Args         []gqlField `json:"args"`
}
type gqlType struct {
	Name        string     `json:"name"`
	Kind        string     `json:"kind"`
	Description string     `json:"description"`
	Fields      []gqlField `json:"fields"`
	InputFields []gqlField `json:"inputFields"`
	EnumValues  []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"enumValues"`
}
type deviceSchema struct {
	Types           map[string]gqlType
	Query, Mutation string
}

func fieldNamed(fields []gqlField, name string) (gqlField, bool) {
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
	}
	return gqlField{}, false
}

func deviceQuery(ctx context.Context, c *Client, query string, vars map[string]any) (map[string]any, error) {
	var res struct {
		Data   map[string]any `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := c.DoQuery(ctx, query, vars, &res); err != nil {
		return nil, err
	}
	if len(res.Errors) > 0 {
		messages := []string{}
		for _, e := range res.Errors {
			messages = append(messages, e.Message)
		}
		return nil, fmt.Errorf("GraphQL: %s", strings.Join(messages, "; "))
	}
	if res.Data == nil {
		return nil, fmt.Errorf("GraphQL response contains no data")
	}
	return res.Data, nil
}
func loadDeviceSchema(ctx context.Context, c *Client) (*deviceSchema, error) {
	ref := "kind name"
	for i := 0; i < 7; i++ {
		ref = "kind name ofType { " + ref + " }"
	}
	q := `query DeviceSchema { __schema { queryType { name } mutationType { name } types { kind name description fields { name description args { name description defaultValue type { ` + ref + ` } } type { ` + ref + ` } } inputFields { name description defaultValue type { ` + ref + ` } } enumValues { name description } } } }`
	data, err := deviceQuery(ctx, c, q, nil)
	if err != nil {
		return nil, fmt.Errorf("device discovery requires GraphQL introspection: %w", err)
	}
	b, _ := json.Marshal(data["__schema"])
	var raw struct {
		QueryType, MutationType struct{ Name string }
		Types                   []gqlType
	}
	if err = json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	s := &deviceSchema{Types: map[string]gqlType{}, Query: raw.QueryType.Name, Mutation: raw.MutationType.Name}
	for _, t := range raw.Types {
		s.Types[t.Name] = t
	}
	if s.Query == "" {
		return nil, fmt.Errorf("broker returned no introspection schema")
	}
	return s, nil
}
func (s *deviceSchema) operation(root, op string) (gqlField, error) {
	r, ok := fieldNamed(s.Types[s.Mutation].Fields, root)
	if !ok {
		return gqlField{}, fmt.Errorf("device API %s is unavailable on this broker", root)
	}
	f, ok := fieldNamed(s.Types[r.Type.base()].Fields, op)
	if !ok {
		return f, fmt.Errorf("operation %s.%s is unavailable on this broker", root, op)
	}
	return f, nil
}
func (s *deviceSchema) validate(ref gqlRef, v any, path string) error {
	if ref.Kind == "NON_NULL" {
		if v == nil {
			return fmt.Errorf("%s: required value is missing", path)
		}
		return s.validate(*ref.OfType, v, path)
	}
	if v == nil {
		return nil
	}
	if ref.Kind == "LIST" {
		list, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array", path)
		}
		for i, item := range list {
			if err := s.validate(*ref.OfType, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	}
	t := s.Types[ref.Name]
	switch t.Kind {
	case "INPUT_OBJECT":
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		keys := []string{}
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if _, ok := fieldNamed(t.InputFields, key); !ok {
				return fmt.Errorf("%s.%s: unknown field", path, key)
			}
		}
		for _, f := range t.InputFields {
			value, exists := obj[f.Name]
			if !exists && f.DefaultValue != nil {
				continue
			}
			if err := s.validate(f.Type, value, path+"."+f.Name); err != nil {
				return err
			}
		}
	case "ENUM":
		str, ok := v.(string)
		if ok {
			for _, e := range t.EnumValues {
				if e.Name == str {
					return nil
				}
			}
		}
		return fmt.Errorf("%s: invalid %s enum value", path, ref.Name)
	default:
		valid := true
		switch ref.Name {
		case "String", "ID":
			_, valid = v.(string)
		case "Boolean":
			_, valid = v.(bool)
		case "Int", "Long", "Float":
			n, ok := v.(float64)
			valid = ok
			if ok && ref.Name != "Float" {
				valid = math.Trunc(n) == n
			}
			if ok && ref.Name == "Int" {
				valid = valid && n >= -2147483648 && n <= 2147483647
			}
		}
		if !valid {
			return fmt.Errorf("%s: expected %s", path, ref.Name)
		}
	}
	return nil
}

// Projection applies only to previously stored configuration, never to user input.
func (s *deviceSchema) project(ref gqlRef, v any) any {
	if ref.OfType != nil {
		if ref.Kind == "LIST" {
			if list, ok := v.([]any); ok {
				out := []any{}
				for _, x := range list {
					out = append(out, s.project(*ref.OfType, x))
				}
				return out
			}
		}
		return s.project(*ref.OfType, v)
	}
	if s.Types[ref.Name].Kind != "INPUT_OBJECT" {
		return v
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := map[string]any{}
	for _, f := range s.Types[ref.Name].InputFields {
		if x, ok := obj[f.Name]; ok {
			out[f.Name] = s.project(f.Type, x)
		}
	}
	return out
}
func (s *deviceSchema) template(ref gqlRef, depth int) any {
	if depth > 12 {
		return nil
	}
	if ref.Kind == "NON_NULL" {
		return s.template(*ref.OfType, depth+1)
	}
	if ref.Kind == "LIST" {
		return []any{}
	}
	t := s.Types[ref.Name]
	if t.Kind == "INPUT_OBJECT" {
		obj := map[string]any{}
		for _, f := range t.InputFields {
			if f.Type.Kind == "NON_NULL" && f.DefaultValue == nil {
				obj[f.Name] = s.template(f.Type, depth+1)
			}
		}
		return obj
	}
	if t.Kind == "ENUM" && len(t.EnumValues) > 0 {
		return t.EnumValues[0].Name
	}
	switch ref.Name {
	case "Boolean":
		return false
	case "Int", "Long", "Float":
		return 0
	case "JSON":
		return map[string]any{}
	default:
		return ""
	}
}
func (s *deviceSchema) inputTypes(ref gqlRef, out map[string]gqlType) {
	name := ref.base()
	if _, ok := out[name]; ok {
		return
	}
	t, ok := s.Types[name]
	if !ok {
		return
	}
	out[name] = t
	for _, f := range t.InputFields {
		s.inputTypes(f.Type, out)
	}
}

// Only read identity and operational state; do not recursively fetch config/secrets.
func (s *deviceSchema) statusSelection(ref gqlRef, depth int) string {
	if depth > 4 {
		return ""
	}
	t := s.Types[ref.base()]
	parts := []string{}
	for _, f := range t.Fields {
		switch f.Name {
		case "name", "nodeId", "enabled", "connected", "isConnected", "status", "state", "connectionState", "connectionStatus", "lastError", "error", "errors", "message", "running", "isRunning", "isOnCurrentNode", "metrics", "messagesIn", "messagesOut", "timestamp":
		default:
			continue
		}
		required := false
		for _, a := range f.Args {
			if a.Type.Kind == "NON_NULL" && a.DefaultValue == nil {
				required = true
			}
		}
		if required {
			continue
		}
		kind := s.Types[f.Type.base()].Kind
		if kind == "OBJECT" {
			sub := s.statusSelection(f.Type, depth+1)
			if sub != "" {
				parts = append(parts, f.Name+" { "+sub+" }")
			}
		} else {
			parts = append(parts, f.Name)
		}
	}
	return strings.Join(parts, " ")
}
