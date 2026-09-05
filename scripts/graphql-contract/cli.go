package main

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	gqlsyntax "github.com/vektah/gqlparser/v2/parser"
)

// Validate complete literal operations against Full Broker. Dynamic device operations
// are checked by CLI contract tests; Edge intentionally supports only a subset.
func checkCLIOperations(root string, c *contract) error {
	paths, e := filepath.Glob(filepath.Join(root, "cli/*.go"))
	if e != nil {
		return e
	}
	checked := 0
	failures := []string{}
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		set := token.NewFileSet()
		file, e := parser.ParseFile(set, p, nil, 0)
		if e != nil {
			return e
		}
		goast.Inspect(file, func(n goast.Node) bool {
			lit, ok := n.(*goast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, e := strconv.Unquote(lit.Value)
			if e != nil {
				return true
			}
			q := strings.TrimSpace(value)
			if !strings.HasPrefix(q, "query") && !strings.HasPrefix(q, "mutation") && !strings.HasPrefix(q, "subscription") {
				return true
			}
			parsed, e := gqlsyntax.ParseQuery(&ast.Source{Name: p, Input: q})
			if e != nil || len(parsed.Operations) == 0 {
				return true
			}
			checked++
			if _, errs := gqlparser.LoadQuery(c.Schema, q); len(errs) > 0 {
				failures = append(failures, fmt.Sprintf("%s: %s", set.Position(lit.Pos()), errs))
			}
			return true
		})
	}
	if checked == 0 {
		return fmt.Errorf("no complete CLI GraphQL operations found")
	}
	if len(failures) > 0 {
		return fmt.Errorf("CLI GraphQL operations incompatible with Full Broker:\n%s", strings.Join(failures, "\n"))
	}
	fmt.Printf("Validated %d literal CLI GraphQL operations against Full Broker\n", checked)
	return nil
}
