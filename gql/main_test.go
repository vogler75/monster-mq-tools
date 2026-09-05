package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func parseSchemaString(t *testing.T, name, sdl string) *ast.Schema {
	t.Helper()
	s, err := gqlparser.LoadSchema(&ast.Source{Name: name, Input: sdl})
	if err != nil {
		t.Fatalf("failed to parse schema %s: %v", name, err)
	}
	return s
}

func TestCompareSchemasInMemory(t *testing.T) {
	sdl1 := `
type Query {
  device(id: ID!): Device
  onlyIn1: String
}
type Mutation {
  save(id: ID!): Boolean!
}
type Device {
  id: ID!
  name: String!
  val: Int
}
enum Status {
  ACTIVE
  INACTIVE
}
`
	sdl2 := `
type Query {
  device(id: ID!): Device
  onlyIn2: Int
}
type Mutation {
  save(id: ID!, force: Boolean): Boolean
}
type Device {
  id: ID!
  name: String
  extra: Float
}
enum Status {
  ACTIVE
  PENDING
}
type ExtraType {
  data: String
}
`
	s1 := parseSchemaString(t, "schema1.gql", sdl1)
	s2 := parseSchemaString(t, "schema2.gql", sdl2)

	rep := compareSchemas(s1, s2, "schema1.gql", "schema2.gql", "")

	if !rep.HasDifferences() {
		t.Fatalf("expected differences, got none")
	}

	// Extra type in s2
	foundExtra := false
	for _, name := range rep.OnlyIn2 {
		if name == "ExtraType" {
			foundExtra = true
			break
		}
	}
	if !foundExtra {
		t.Errorf("expected ExtraType only in schema 2")
	}

	// Type differences in Device
	var devDiff *TypeDiff
	for i := range rep.TypeDifferences {
		if rep.TypeDifferences[i].Name == "Device" {
			devDiff = &rep.TypeDifferences[i]
			break
		}
	}
	if devDiff == nil {
		t.Fatalf("expected diff for Device")
	}
	if len(devDiff.OnlyIn1) != 1 || devDiff.OnlyIn1[0] != "val" {
		t.Errorf("expected val only in 1, got %v", devDiff.OnlyIn1)
	}
	if len(devDiff.OnlyIn2) != 1 || devDiff.OnlyIn2[0] != "extra" {
		t.Errorf("expected extra only in 2, got %v", devDiff.OnlyIn2)
	}

	// Field diffs for name: String! vs String
	foundNameMismatch := false
	for _, fd := range devDiff.FieldDiffs {
		if fd.Name == "name" && fd.Issue == "return type mismatch" {
			foundNameMismatch = true
			break
		}
	}
	if !foundNameMismatch {
		t.Errorf("expected name return type mismatch in Device")
	}

	// Enum diffs in Status
	var statusDiff *TypeDiff
	for i := range rep.TypeDifferences {
		if rep.TypeDifferences[i].Name == "Status" {
			statusDiff = &rep.TypeDifferences[i]
			break
		}
	}
	if statusDiff == nil {
		t.Fatalf("expected diff for Status enum")
	}
	if len(statusDiff.ValuesOnly1) != 1 || statusDiff.ValuesOnly1[0] != "INACTIVE" {
		t.Errorf("expected INACTIVE only in 1, got %v", statusDiff.ValuesOnly1)
	}
	if len(statusDiff.ValuesOnly2) != 1 || statusDiff.ValuesOnly2[0] != "PENDING" {
		t.Errorf("expected PENDING only in 2, got %v", statusDiff.ValuesOnly2)
	}
}

func TestCompareActualSchemas(t *testing.T) {
	s1, err := loadSchemaFile("main.gql")
	if err != nil {
		t.Fatalf("failed to load main.gql: %v", err)
	}
	s2, err := loadSchemaFile("edge.gql")
	if err != nil {
		t.Fatalf("failed to load edge.gql: %v", err)
	}

	rep := compareSchemas(s1, s2, "main.gql", "edge.gql", "")

	if rep.TotalTypes1 == 0 || rep.TotalTypes2 == 0 {
		t.Fatalf("empty schema types parsed")
	}
	if rep.SharedTypes == 0 {
		t.Fatalf("expected shared types, got 0")
	}
	if len(rep.OnlyIn1) == 0 {
		t.Fatalf("expected types only in main.gql")
	}
	if len(rep.OnlyIn2) == 0 {
		t.Fatalf("expected types only in edge.gql (e.g. Redfish)")
	}

	// Verify JSON serialization
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(rep); err != nil {
		t.Fatalf("failed to encode json report: %v", err)
	}
}

func TestFilterByType(t *testing.T) {
	s1, err := loadSchemaFile("main.gql")
	if err != nil {
		t.Fatalf("failed to load main.gql: %v", err)
	}
	s2, err := loadSchemaFile("edge.gql")
	if err != nil {
		t.Fatalf("failed to load edge.gql: %v", err)
	}

	rep := compareSchemas(s1, s2, "main.gql", "edge.gql", "Query")
	if rep.SharedTypes != 1 {
		t.Fatalf("expected 1 shared type for Query filter, got %d", rep.SharedTypes)
	}
	if len(rep.TypeDifferences) != 1 || rep.TypeDifferences[0].Name != "Query" {
		t.Fatalf("expected only Query in differences")
	}
}

func TestMarkdownAndTextOutputNoPanic(t *testing.T) {
	s1, _ := loadSchemaFile("main.gql")
	s2, _ := loadSchemaFile("edge.gql")
	rep := compareSchemas(s1, s2, "main.gql", "edge.gql", "Query")

	// redirect stdout temporarily
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printMarkdown(rep, false)
	printMarkdown(rep, true)
	printText(rep, false)
	printText(rep, true)

	w.Close()
	os.Stdout = old
	io.ReadAll(r)
}
