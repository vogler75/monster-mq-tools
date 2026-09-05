package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// Ignore documentation/order and compare API shape. Source comments are not live descriptions.
func shape(schema map[string]any) map[string]string {
	out := map[string]string{}
	for _, n := range []string{"queryType", "mutationType", "subscriptionType"} {
		if root, ok := schema[n].(map[string]any); ok {
			out[n] = fmt.Sprint(root["name"])
		}
	}
	types, _ := schema["types"].([]any)
	for _, v := range types {
		t, _ := v.(map[string]any)
		name, _ := t["name"].(string)
		if strings.HasPrefix(name, "__") || name == "String" || name == "Int" || name == "Float" || name == "Boolean" || name == "ID" {
			continue
		}
		out[name] = fmt.Sprint(t["kind"])
		for _, collection := range []string{"fields", "inputFields"} {
			fields, _ := t[collection].([]any)
			for _, v := range fields {
				f, _ := v.(map[string]any)
				fn := fmt.Sprint(f["name"])
				if strings.HasPrefix(fn, "__") {
					continue
				}
				path := name + "." + fn
				out[path] = refString(f["type"])
				if d, ok := f["defaultValue"].(string); ok {
					out[path+" default"] = d
				}
				if f["isDeprecated"] == true {
					out[path+" deprecated"] = "true"
				}
				args, _ := f["args"].([]any)
				for _, v := range args {
					a, _ := v.(map[string]any)
					key := path + "(" + fmt.Sprint(a["name"]) + ")"
					out[key] = refString(a["type"])
					if d, ok := a["defaultValue"].(string); ok {
						out[key+" default"] = d
					}
				}
			}
		}
		for _, collection := range []string{"enumValues", "interfaces", "possibleTypes"} {
			list, _ := t[collection].([]any)
			for _, v := range list {
				item, _ := v.(map[string]any)
				out[name+" "+collection+" "+fmt.Sprint(item["name"])] = "present"
			}
		}
	}
	return out
}
func refString(v any) string {
	r, _ := v.(map[string]any)
	switch r["kind"] {
	case "NON_NULL":
		return refString(r["ofType"]) + "!"
	case "LIST":
		return "[" + refString(r["ofType"]) + "]"
	}
	return fmt.Sprint(r["name"])
}
func compareShapes(expected, actual map[string]string) error {
	changes := []string{}
	for _, k := range names(expected) {
		if actual[k] != expected[k] {
			changes = append(changes, fmt.Sprintf("%s: source=%q live=%q", k, expected[k], actual[k]))
		}
	}
	for _, k := range names(actual) {
		if _, ok := expected[k]; !ok {
			changes = append(changes, fmt.Sprintf("%s: live-only %q", k, actual[k]))
		}
	}
	sort.Strings(changes)
	if len(changes) > 0 {
		return fmt.Errorf("live GraphQL contract differs from source:\n  %s", strings.Join(changes, "\n  "))
	}
	return nil
}
func checkLive(endpoint string, c *contract) error {
	ref := "kind name"
	for i := 0; i < 8; i++ {
		ref = "kind name ofType { " + ref + " }"
	}
	query := `query ContractIntrospection { __schema { queryType { name } mutationType { name } subscriptionType { name } types { kind name fields(includeDeprecated: true) { name isDeprecated args { name defaultValue type { ` + ref + ` } } type { ` + ref + ` } } inputFields { name defaultValue type { ` + ref + ` } } enumValues(includeDeprecated: true) { name } interfaces { name } possibleTypes { name } } } }`
	body, _ := json.Marshal(map[string]any{"query": query})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("GRAPHQL_CONTRACT_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	res, e := client.Do(req)
	if e != nil {
		return fmt.Errorf("live introspection request failed: %w", e)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("live introspection returned HTTP %d", res.StatusCode)
	}
	var response struct {
		Data struct {
			Schema map[string]any `json:"__schema"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if e := json.NewDecoder(io.LimitReader(res.Body, 32<<20)).Decode(&response); e != nil {
		return e
	}
	if len(response.Errors) > 0 {
		return fmt.Errorf("live introspection failed: %s", response.Errors[0].Message)
	}
	if response.Data.Schema == nil {
		return fmt.Errorf("live introspection returned no schema")
	}
	// Normalize the generated map through JSON, just like a real response.
	var source map[string]any
	json.Unmarshal(jsonBytes(introspection(c, nil, nil)), &source)
	expected := source["data"].(map[string]any)["__schema"].(map[string]any)
	if e := compareShapes(shape(expected), shape(response.Data.Schema)); e != nil {
		return e
	}
	fmt.Println("Live GraphQL shape matches source (resolver behavior not tested)")
	return nil
}
