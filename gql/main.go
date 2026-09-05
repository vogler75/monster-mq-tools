package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// FieldDiff captures differences in a specific field or argument.
type FieldDiff struct {
	Name      string   `json:"name"`
	Issue     string   `json:"issue"`
	Details   string   `json:"details,omitempty"`
	Arguments []string `json:"arguments,omitempty"`
}

// TypeDiff captures differences between a type defined in both schemas.
type TypeDiff struct {
	Name        string      `json:"name"`
	Kind1       string      `json:"kind1"`
	Kind2       string      `json:"kind2"`
	KindMatch   bool        `json:"kindMatch"`
	OnlyIn1     []string    `json:"onlyIn1,omitempty"`
	OnlyIn2     []string    `json:"onlyIn2,omitempty"`
	FieldDiffs  []FieldDiff `json:"fieldDiffs,omitempty"`
	ValuesOnly1 []string    `json:"valuesOnly1,omitempty"`
	ValuesOnly2 []string    `json:"valuesOnly2,omitempty"`
}

func (td TypeDiff) HasDifferences() bool {
	return !td.KindMatch || len(td.OnlyIn1) > 0 || len(td.OnlyIn2) > 0 ||
		len(td.FieldDiffs) > 0 || len(td.ValuesOnly1) > 0 || len(td.ValuesOnly2) > 0
}

// ComparisonReport is the overall comparison structure.
type ComparisonReport struct {
	Schema1Path string   `json:"schema1"`
	Schema2Path string   `json:"schema2"`
	Schema1Name string   `json:"schema1Name"`
	Schema2Name string   `json:"schema2Name"`
	TotalTypes1 int      `json:"totalTypes1"`
	TotalTypes2 int      `json:"totalTypes2"`
	SharedTypes int      `json:"sharedTypes"`
	OnlyIn1     []string `json:"onlyIn1"`
	OnlyIn2     []string `json:"onlyIn2"`

	RootQuery1        string `json:"rootQuery1,omitempty"`
	RootQuery2        string `json:"rootQuery2,omitempty"`
	RootMutation1     string `json:"rootMutation1,omitempty"`
	RootMutation2     string `json:"rootMutation2,omitempty"`
	RootSubscription1 string `json:"rootSubscription1,omitempty"`
	RootSubscription2 string `json:"rootSubscription2,omitempty"`

	TypeDifferences []TypeDiff `json:"typeDifferences,omitempty"`
}

func (r *ComparisonReport) HasDifferences() bool {
	return len(r.OnlyIn1) > 0 || len(r.OnlyIn2) > 0 || len(r.TypeDifferences) > 0 ||
		r.RootQuery1 != r.RootQuery2 || r.RootMutation1 != r.RootMutation2 || r.RootSubscription1 != r.RootSubscription2
}

func loadSchemaFile(path string) (*ast.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	src := &ast.Source{
		Name:  filepath.Base(path),
		Input: string(data),
	}
	schema, err := gqlparser.LoadSchema(src)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return schema, nil
}

func formatType(t *ast.Type) string {
	if t == nil {
		return ""
	}
	return t.String()
}

func compareSchemas(s1, s2 *ast.Schema, path1, path2, filterType string) *ComparisonReport {
	name1 := filepath.Base(path1)
	name2 := filepath.Base(path2)

	rep := &ComparisonReport{
		Schema1Path: path1,
		Schema2Path: path2,
		Schema1Name: name1,
		Schema2Name: name2,
		OnlyIn1:     []string{},
		OnlyIn2:     []string{},
	}

	if s1.Query != nil {
		rep.RootQuery1 = s1.Query.Name
	}
	if s2.Query != nil {
		rep.RootQuery2 = s2.Query.Name
	}
	if s1.Mutation != nil {
		rep.RootMutation1 = s1.Mutation.Name
	}
	if s2.Mutation != nil {
		rep.RootMutation2 = s2.Mutation.Name
	}
	if s1.Subscription != nil {
		rep.RootSubscription1 = s1.Subscription.Name
	}
	if s2.Subscription != nil {
		rep.RootSubscription2 = s2.Subscription.Name
	}

	types1 := map[string]*ast.Definition{}
	for name, def := range s1.Types {
		if !def.BuiltIn && !strings.HasPrefix(name, "__") {
			types1[name] = def
		}
	}
	types2 := map[string]*ast.Definition{}
	for name, def := range s2.Types {
		if !def.BuiltIn && !strings.HasPrefix(name, "__") {
			types2[name] = def
		}
	}

	rep.TotalTypes1 = len(types1)
	rep.TotalTypes2 = len(types2)

	allNames := map[string]bool{}
	for n := range types1 {
		allNames[n] = true
	}
	for n := range types2 {
		allNames[n] = true
	}

	sortedNames := make([]string, 0, len(allNames))
	for n := range allNames {
		sortedNames = append(sortedNames, n)
	}
	sort.Strings(sortedNames)

	for _, name := range sortedNames {
		if filterType != "" && !strings.EqualFold(name, filterType) {
			continue
		}

		d1, ok1 := types1[name]
		d2, ok2 := types2[name]

		if ok1 && !ok2 {
			rep.OnlyIn1 = append(rep.OnlyIn1, name)
			continue
		}
		if !ok1 && ok2 {
			rep.OnlyIn2 = append(rep.OnlyIn2, name)
			continue
		}

		rep.SharedTypes++
		td := compareTypeDefinitions(d1, d2)
		if td.HasDifferences() {
			rep.TypeDifferences = append(rep.TypeDifferences, td)
		}
	}

	return rep
}

func compareTypeDefinitions(d1, d2 *ast.Definition) TypeDiff {
	td := TypeDiff{
		Name:        d1.Name,
		Kind1:       string(d1.Kind),
		Kind2:       string(d2.Kind),
		KindMatch:   d1.Kind == d2.Kind,
		OnlyIn1:     []string{},
		OnlyIn2:     []string{},
		FieldDiffs:  []FieldDiff{},
		ValuesOnly1: []string{},
		ValuesOnly2: []string{},
	}

	if !td.KindMatch {
		return td
	}

	switch d1.Kind {
	case ast.Object, ast.Interface:
		fields1 := map[string]*ast.FieldDefinition{}
		for _, f := range d1.Fields {
			if !strings.HasPrefix(f.Name, "__") {
				fields1[f.Name] = f
			}
		}
		fields2 := map[string]*ast.FieldDefinition{}
		for _, f := range d2.Fields {
			if !strings.HasPrefix(f.Name, "__") {
				fields2[f.Name] = f
			}
		}

		for fname := range fields1 {
			if _, ok := fields2[fname]; !ok {
				td.OnlyIn1 = append(td.OnlyIn1, fname)
			}
		}
		for fname := range fields2 {
			if _, ok := fields1[fname]; !ok {
				td.OnlyIn2 = append(td.OnlyIn2, fname)
			}
		}
		sort.Strings(td.OnlyIn1)
		sort.Strings(td.OnlyIn2)

		for fname, f1 := range fields1 {
			f2, ok := fields2[fname]
			if !ok {
				continue
			}

			fd := FieldDiff{Name: fname}
			t1Str := formatType(f1.Type)
			t2Str := formatType(f2.Type)
			if t1Str != t2Str {
				fd.Issue = "return type mismatch"
				fd.Details = fmt.Sprintf("schema1: %s, schema2: %s", t1Str, t2Str)
			}

			// Compare arguments
			args1 := map[string]*ast.ArgumentDefinition{}
			for _, a := range f1.Arguments {
				args1[a.Name] = a
			}
			args2 := map[string]*ast.ArgumentDefinition{}
			for _, a := range f2.Arguments {
				args2[a.Name] = a
			}

			argIssues := []string{}
			for aname, a1 := range args1 {
				a2, ok := args2[aname]
				if !ok {
					argIssues = append(argIssues, fmt.Sprintf("arg %s only in schema1", aname))
					continue
				}
				if a1.Type.String() != a2.Type.String() {
					argIssues = append(argIssues, fmt.Sprintf("arg %s type mismatch: %s vs %s", aname, a1.Type.String(), a2.Type.String()))
				}
			}
			for aname := range args2 {
				if _, ok := args1[aname]; !ok {
					argIssues = append(argIssues, fmt.Sprintf("arg %s only in schema2", aname))
				}
			}
			sort.Strings(argIssues)

			if len(argIssues) > 0 {
				if fd.Issue == "" {
					fd.Issue = "argument mismatch"
				} else {
					fd.Issue += " & argument mismatch"
				}
				fd.Arguments = argIssues
			}

			if fd.Issue != "" {
				td.FieldDiffs = append(td.FieldDiffs, fd)
			}
		}
		sort.Slice(td.FieldDiffs, func(i, j int) bool { return td.FieldDiffs[i].Name < td.FieldDiffs[j].Name })

	case ast.InputObject:
		fields1 := map[string]*ast.FieldDefinition{}
		for _, f := range d1.Fields {
			fields1[f.Name] = f
		}
		fields2 := map[string]*ast.FieldDefinition{}
		for _, f := range d2.Fields {
			fields2[f.Name] = f
		}

		for fname := range fields1 {
			if _, ok := fields2[fname]; !ok {
				td.OnlyIn1 = append(td.OnlyIn1, fname)
			}
		}
		for fname := range fields2 {
			if _, ok := fields1[fname]; !ok {
				td.OnlyIn2 = append(td.OnlyIn2, fname)
			}
		}
		sort.Strings(td.OnlyIn1)
		sort.Strings(td.OnlyIn2)

		for fname, f1 := range fields1 {
			f2, ok := fields2[fname]
			if !ok {
				continue
			}
			t1Str := formatType(f1.Type)
			t2Str := formatType(f2.Type)
			if t1Str != t2Str {
				td.FieldDiffs = append(td.FieldDiffs, FieldDiff{
					Name:    fname,
					Issue:   "type mismatch",
					Details: fmt.Sprintf("schema1: %s, schema2: %s", t1Str, t2Str),
				})
			}
		}
		sort.Slice(td.FieldDiffs, func(i, j int) bool { return td.FieldDiffs[i].Name < td.FieldDiffs[j].Name })

	case ast.Enum:
		vals1 := map[string]bool{}
		for _, v := range d1.EnumValues {
			vals1[v.Name] = true
		}
		vals2 := map[string]bool{}
		for _, v := range d2.EnumValues {
			vals2[v.Name] = true
		}
		for v := range vals1 {
			if !vals2[v] {
				td.ValuesOnly1 = append(td.ValuesOnly1, v)
			}
		}
		for v := range vals2 {
			if !vals1[v] {
				td.ValuesOnly2 = append(td.ValuesOnly2, v)
			}
		}
		sort.Strings(td.ValuesOnly1)
		sort.Strings(td.ValuesOnly2)

	case ast.Union:
		types1 := map[string]bool{}
		for _, t := range d1.Types {
			types1[t] = true
		}
		types2 := map[string]bool{}
		for _, t := range d2.Types {
			types2[t] = true
		}
		for t := range types1 {
			if !types2[t] {
				td.ValuesOnly1 = append(td.ValuesOnly1, t)
			}
		}
		for t := range types2 {
			if !types1[t] {
				td.ValuesOnly2 = append(td.ValuesOnly2, t)
			}
		}
		sort.Strings(td.ValuesOnly1)
		sort.Strings(td.ValuesOnly2)
	}

	return td
}

func printText(r *ComparisonReport, summaryOnly bool) {
	fmt.Println("================================================================================")
	fmt.Println("GraphQL Schema Comparison")
	fmt.Println("================================================================================")
	fmt.Printf("Schema 1: %s (%d types)\n", r.Schema1Path, r.TotalTypes1)
	fmt.Printf("Schema 2: %s (%d types)\n", r.Schema2Path, r.TotalTypes2)
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("Shared Types:            %d\n", r.SharedTypes)
	fmt.Printf("Types Only in Schema 1:  %d\n", len(r.OnlyIn1))
	fmt.Printf("Types Only in Schema 2:  %d\n", len(r.OnlyIn2))
	fmt.Printf("Shared with Differences: %d\n", len(r.TypeDifferences))
	fmt.Println("--------------------------------------------------------------------------------")

	// Root operations
	fmt.Println("Root Operations:")
	fmt.Printf("  Query:        Schema 1 = %-15s | Schema 2 = %-15s\n", r.RootQuery1, r.RootQuery2)
	fmt.Printf("  Mutation:     Schema 1 = %-15s | Schema 2 = %-15s\n", r.RootMutation1, r.RootMutation2)
	fmt.Printf("  Subscription: Schema 1 = %-15s | Schema 2 = %-15s\n", r.RootSubscription1, r.RootSubscription2)
	fmt.Println("--------------------------------------------------------------------------------")

	if len(r.OnlyIn1) > 0 {
		fmt.Printf("\nTypes Only in Schema 1 (%s):\n", r.Schema1Name)
		for _, t := range r.OnlyIn1 {
			fmt.Printf("  - %s\n", t)
		}
	}

	if len(r.OnlyIn2) > 0 {
		fmt.Printf("\nTypes Only in Schema 2 (%s):\n", r.Schema2Name)
		for _, t := range r.OnlyIn2 {
			fmt.Printf("  - %s\n", t)
		}
	}

	if summaryOnly {
		return
	}

	if len(r.TypeDifferences) > 0 {
		fmt.Printf("\nDetailed Differences in Shared Types (%d types):\n", len(r.TypeDifferences))
		for _, td := range r.TypeDifferences {
			fmt.Printf("\n  • %s (%s):\n", td.Name, td.Kind1)
			if !td.KindMatch {
				fmt.Printf("      Kind mismatch: %s vs %s\n", td.Kind1, td.Kind2)
			}
			if len(td.OnlyIn1) > 0 {
				fmt.Printf("      Fields only in %s: %s\n", r.Schema1Name, strings.Join(td.OnlyIn1, ", "))
			}
			if len(td.OnlyIn2) > 0 {
				fmt.Printf("      Fields only in %s: %s\n", r.Schema2Name, strings.Join(td.OnlyIn2, ", "))
			}
			for _, fd := range td.FieldDiffs {
				fmt.Printf("      Field %s: %s", fd.Name, fd.Issue)
				if fd.Details != "" {
					fmt.Printf(" (%s)", fd.Details)
				}
				fmt.Println()
				for _, arg := range fd.Arguments {
					fmt.Printf("        - %s\n", arg)
				}
			}
			if len(td.ValuesOnly1) > 0 {
				fmt.Printf("      Values only in %s: %s\n", r.Schema1Name, strings.Join(td.ValuesOnly1, ", "))
			}
			if len(td.ValuesOnly2) > 0 {
				fmt.Printf("      Values only in %s: %s\n", r.Schema2Name, strings.Join(td.ValuesOnly2, ", "))
			}
		}
	} else {
		fmt.Println("\n✓ No structural differences detected among shared types.")
	}
	fmt.Println("================================================================================")
}

func printMarkdown(r *ComparisonReport, summaryOnly bool) {
	fmt.Println("# GraphQL Schema Comparison")
	fmt.Println()
	fmt.Printf("| Metric | Schema 1 (`%s`) | Schema 2 (`%s`) |\n", r.Schema1Name, r.Schema2Name)
	fmt.Println("|---|---|---|")
	fmt.Printf("| **File Path** | `%s` | `%s` |\n", r.Schema1Path, r.Schema2Path)
	fmt.Printf("| **Total Types** | %d | %d |\n", r.TotalTypes1, r.TotalTypes2)
	fmt.Printf("| **Query Root** | `%s` | `%s` |\n", r.RootQuery1, r.RootQuery2)
	fmt.Printf("| **Mutation Root** | `%s` | `%s` |\n", r.RootMutation1, r.RootMutation2)
	fmt.Printf("| **Subscription Root** | `%s` | `%s` |\n", r.RootSubscription1, r.RootSubscription2)
	fmt.Println()
	fmt.Println("### Inventory Summary")
	fmt.Println()
	fmt.Printf("- **Shared Types:** %d\n", r.SharedTypes)
	fmt.Printf("- **Exclusive to Schema 1:** %d\n", len(r.OnlyIn1))
	fmt.Printf("- **Exclusive to Schema 2:** %d\n", len(r.OnlyIn2))
	fmt.Printf("- **Shared with Differences:** %d\n", len(r.TypeDifferences))
	fmt.Println()

	if len(r.OnlyIn1) > 0 {
		fmt.Printf("### Types Exclusive to `%s` (%d)\n\n", r.Schema1Name, len(r.OnlyIn1))
		for _, t := range r.OnlyIn1 {
			fmt.Printf("- `%s`\n", t)
		}
		fmt.Println()
	}

	if len(r.OnlyIn2) > 0 {
		fmt.Printf("### Types Exclusive to `%s` (%d)\n\n", r.Schema2Name, len(r.OnlyIn2))
		for _, t := range r.OnlyIn2 {
			fmt.Printf("- `%s`\n", t)
		}
		fmt.Println()
	}

	if summaryOnly {
		return
	}

	if len(r.TypeDifferences) > 0 {
		fmt.Printf("### Differences in Shared Types (%d)\n\n", len(r.TypeDifferences))
		for _, td := range r.TypeDifferences {
			fmt.Printf("#### `%s` (%s)\n\n", td.Name, td.Kind1)
			if !td.KindMatch {
				fmt.Printf("- **Kind Mismatch:** `%s` vs `%s`\n", td.Kind1, td.Kind2)
			}
			if len(td.OnlyIn1) > 0 {
				fmt.Printf("- **Fields only in `%s`:** `%s`\n", r.Schema1Name, strings.Join(td.OnlyIn1, "`, `"))
			}
			if len(td.OnlyIn2) > 0 {
				fmt.Printf("- **Fields only in `%s`:** `%s`\n", r.Schema2Name, strings.Join(td.OnlyIn2, "`, `"))
			}
			for _, fd := range td.FieldDiffs {
				fmt.Printf("- **Field `%s`:** %s", fd.Name, fd.Issue)
				if fd.Details != "" {
					fmt.Printf(" (%s)", fd.Details)
				}
				fmt.Println()
				for _, a := range fd.Arguments {
					fmt.Printf("  - %s\n", a)
				}
			}
			if len(td.ValuesOnly1) > 0 {
				fmt.Printf("- **Values only in `%s`:** `%s`\n", r.Schema1Name, strings.Join(td.ValuesOnly1, "`, `"))
			}
			if len(td.ValuesOnly2) > 0 {
				fmt.Printf("- **Values only in `%s`:** `%s`\n", r.Schema2Name, strings.Join(td.ValuesOnly2, "`, `"))
			}
			fmt.Println()
		}
	}
}

func main() {
	summaryFlag := flag.Bool("summary", false, "Show high-level comparison summary only")
	typeFlag := flag.String("type", "", "Compare only a specific type (e.g. Query, Device)")
	formatFlag := flag.String("format", "text", "Output format: text, markdown, json")
	failOnDiff := flag.Bool("fail-on-diff", false, "Exit with error code 1 if differences are found")
	flag.Parse()

	args := flag.Args()
	schema1Path := "main.gql"
	schema2Path := "edge.gql"

	if len(args) >= 1 {
		schema1Path = args[0]
	}
	if len(args) >= 2 {
		schema2Path = args[1]
	}

	// Auto-resolve relative to script or current directory if default paths are missing
	if len(args) == 0 {
		for _, dir := range []string{".", "gql", "../gql"} {
			s1 := filepath.Join(dir, "main.gql")
			s2 := filepath.Join(dir, "edge.gql")
			if _, err1 := os.Stat(s1); err1 == nil {
				if _, err2 := os.Stat(s2); err2 == nil {
					schema1Path = s1
					schema2Path = s2
					break
				}
			}
		}
	}

	s1, err := loadSchemaFile(schema1Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	s2, err := loadSchemaFile(schema2Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	report := compareSchemas(s1, s2, schema1Path, schema2Path, *typeFlag)

	switch strings.ToLower(*formatFlag) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding json: %v\n", err)
			os.Exit(2)
		}
	case "markdown", "md":
		printMarkdown(report, *summaryFlag)
	default:
		printText(report, *summaryFlag)
	}

	if *failOnDiff && report.HasDifferences() {
		os.Exit(1)
	}
}
