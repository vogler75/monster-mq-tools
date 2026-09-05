package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testContract(t *testing.T) *contract {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "schema.graphqls")
	if e := os.WriteFile(p, []byte(`
# Read things.
type Query { thing(limit: Int = 10): Thing }
type Mutation { save(input: Input!): Boolean! }
"""A stored thing."""
type Thing { name: String! old: String @deprecated(reason: "Use name") }
input Input { name: String! enabled: Boolean = false }
`), 0644); e != nil {
		t.Fatal(e)
	}
	c, e := readSources(root, []string{p})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestGenerationAndCheck(t *testing.T) {
	c := testContract(t)
	dir := t.TempDir()
	files := outputs(c, dir, "Full Broker")
	if e := writeOutputs(files, false); e != nil {
		t.Fatal(e)
	}
	if e := writeOutputs(outputs(c, dir, "Full Broker"), true); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(dir, "reference.md")
	os.WriteFile(p, []byte("stale"), 0644)
	if e := writeOutputs(files, true); e == nil {
		t.Fatal("stale reference accepted")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "stale" {
		t.Fatal("check modified files")
	}
}
func TestReferenceHasDescriptionsAndComments(t *testing.T) {
	c := testContract(t)
	b := string(markdown(c, "Full Broker"))
	for _, s := range []string{"A stored thing.", "Read things.", "Use name", "Query.thing arguments"} {
		if !strings.Contains(b, s) {
			t.Fatalf("missing %s", s)
		}
	}
}
func TestProjectionUsesRegistry(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "cli"), 0755)
	os.WriteFile(filepath.Join(root, "cli/devices.go"), []byte(`package main
var deviceAdapters = []deviceAdapter{{"New-Client", "newClient", "newClients", "create"}}`), 0644)
	q, m, e := cliRoots(root)
	if e != nil || !q["newClients"] || !m["newClient"] {
		t.Fatalf("registry not read: %v %v %v", q, m, e)
	}
}
func TestMissingOrInvalidSourcesFail(t *testing.T) {
	if _, e := readSources(t.TempDir(), nil); e == nil {
		t.Fatal("empty inputs accepted")
	}
	root := t.TempDir()
	p := filepath.Join(root, "x")
	os.WriteFile(p, []byte(`type Query { broken: Missing }`), 0644)
	if _, e := readSources(root, []string{p}); e == nil {
		t.Fatal("invalid SDL accepted")
	}
}
func TestRuntimeSourceList(t *testing.T) {
	root := t.TempDir()
	loader := filepath.Join(root, "broker/src/main/kotlin/graphql")
	resources := filepath.Join(root, "broker/src/main/resources")
	os.MkdirAll(loader, 0755)
	os.MkdirAll(resources, 0755)
	os.WriteFile(filepath.Join(loader, "GraphQLServer.kt"), []byte("private fun loadSchema(): String {\n val schemaFiles = listOf(\n \"schema-active.graphqls\"\n )\n}"), 0644)
	os.WriteFile(filepath.Join(resources, "schema-active.graphqls"), []byte("type Query { active: Boolean }"), 0644)
	os.WriteFile(filepath.Join(resources, "schema-unused.graphqls"), []byte("this file is not loaded"), 0644)
	c, e := readFull(root)
	if e != nil || len(c.Sources) != 1 {
		t.Fatalf("wrong runtime sources: %v", e)
	}
}
func TestStaticCLIValidation(t *testing.T) {
	c := testContract(t)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "cli"), 0755)
	p := filepath.Join(root, "cli/commands.go")
	os.WriteFile(p, []byte("package main\nvar query = `query { unknown }`"), 0644)
	if e := checkCLIOperations(root, c); e == nil {
		t.Fatal("invalid operation accepted")
	}
	os.WriteFile(p, []byte("package main\nvar query = `query { thing { name } }`"), 0644)
	if e := checkCLIOperations(root, c); e != nil {
		t.Fatal(e)
	}
}
func TestLiveShapeAndAuth(t *testing.T) {
	c := testContract(t)
	t.Setenv("GRAPHQL_CONTRACT_TOKEN", "test-token")
	response := introspection(c, nil, nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth")
		}
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		if !strings.HasPrefix(req["query"], "query ContractIntrospection") {
			t.Error("expected introspection only")
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	if e := checkLive(server.URL, c); e != nil {
		t.Fatal(e)
	}
	response = map[string]any{"data": map[string]any{"__schema": map[string]any{"types": []any{}}}}
	if e := checkLive(server.URL, c); e == nil {
		t.Fatal("missing live schema shape accepted")
	}
}
func TestShapeDetectsTypeChanges(t *testing.T) {
	e := compareShapes(map[string]string{"Input.name": "String!"}, map[string]string{"Input.name": "Int!"})
	if e == nil || !strings.Contains(e.Error(), "Input.name") {
		t.Fatalf("missing useful difference: %v", e)
	}
}
