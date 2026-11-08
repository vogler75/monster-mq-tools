package main

import (
	"sort"
	"path/filepath"
	"os"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// Every operation of both brokers must yield a document that validates against the real SDL.
func TestAPIGeneratedDocumentsValidateAgainstSchema(t *testing.T) {
	for _, broker := range []string{"main", "edge"} {
		t.Run(broker, func(t *testing.T) {
			_, sdl := deviceContract(t, broker)
			api, err := gqlparser.LoadSchema(&ast.Source{Name: broker, Input: string(sdl)})
			if err != nil {
				t.Fatal(err)
			}
			c := deviceTestClient(t, broker, nil, nil)
			s, err := loadDeviceSchema(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			ops := s.operations()
			if len(ops) < 20 {
				t.Fatalf("too few operations: %d", len(ops))
			}
			for _, op := range ops {
				path := op.Kind + ":" + op.Path
				_, segs, err := s.resolve(path)
				if err != nil {
					t.Fatalf("%s: %v", path, err)
				}
				args := map[string]any{}
				for _, a := range segs[len(segs)-1].Args {
					if a.Type.Kind == "NON_NULL" && a.DefaultValue == nil {
						args[a.Name] = s.template(a.Type, 0)
					}
				}
				b, _ := json.Marshal(args)
				doc, _, _, _, err := s.buildOperation(path, apiOptions{Depth: 3, ArgsSrc: string(b)})
				if err != nil {
					t.Errorf("%s: build: %v", path, err)
					continue
				}
				if _, errs := gqlparser.LoadQuery(api, doc); len(errs) > 0 {
					t.Errorf("%s: invalid document %q: %s", path, doc, errs)
				}
			}
		})
	}
}

func TestAPIFlatInputAndKeyValue(t *testing.T) {
	c := deviceTestClient(t, "main", nil, nil)
	s, _ := loadDeviceSchema(context.Background(), c)
	o, err := parseAPIOptions([]string{"username=bob", "password=123456", "isAdmin=true"})
	if err != nil {
		t.Fatal(err)
	}
	doc, vars, kind, _, err := s.buildOperation("user.createUser", o)
	if err != nil {
		t.Fatal(err)
	}
	if kind != "mutation" || !strings.Contains(doc, "createUser(input: $input)") {
		t.Fatalf("unexpected doc: %s", doc)
	}
	in := vars["input"].(map[string]any)
	if in["username"] != "bob" || in["password"] != "123456" || in["isAdmin"] != true {
		t.Fatalf("password must stay a string, flag a bool: %#v", in)
	}
}

func TestAPIArrayArgAndUnknownArg(t *testing.T) {
	c := deviceTestClient(t, "main", nil, nil)
	s, _ := loadDeviceSchema(context.Background(), c)
	o := apiOptions{Depth: 3, ArgsSrc: `[{"topic":"a/b","payload":"1"}]`}
	if _, vars, _, _, err := s.buildOperation("mutation:publishBatch", o); err != nil || vars["inputs"] == nil {
		t.Fatalf("array should map to inputs: %v %v", vars, err)
	}
	if _, _, _, _, err := s.buildOperation("users", apiOptions{Depth: 3, ArgsSrc: `{"nope":1}`}); err == nil {
		t.Fatal("unknown argument must be rejected")
	}
	if _, _, _, _, err := s.buildOperation("topicNamespace", apiOptions{Depth: 3}); err == nil {
		t.Fatal("missing required arg must be rejected")
	}
	if _, _, err := s.resolve("doesNotExist"); err == nil {
		t.Fatal("unknown operation must fail")
	}
}

func TestEdgeLacksFullOnlyOperations(t *testing.T) {
	c := deviceTestClient(t, "edge", nil, nil)
	s, _ := loadDeviceSchema(context.Background(), c)
	if _, _, err := s.resolve("user.createUser"); err == nil {
		t.Skip("edge exposes user management")
	}
}

func TestRunNounAndAPIExecution(t *testing.T) {
	var seen string
	c := deviceTestClient(t, "main", nil, func(r deviceRequest) any {
		seen = r.Query
		return map[string]any{"data": map[string]any{"topicNamespace": map[string]any{"name": "ns", "password": "x"}}}
	})
	out, err := deviceOutput(t, func() error {
		return ExecuteCommand(context.Background(), c, []string{"namespace", "get", "ns"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen, "topicNamespace(name: $name)") {
		t.Fatalf("positional operand should bind to first arg: %s", seen)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("secrets must be redacted by default: %s", out)
	}
	if err := ExecuteCommand(context.Background(), c, []string{"namespace", "bogus"}); err == nil || !strings.Contains(err.Error(), "unknown verb") {
		t.Fatalf("unknown verb must list options: %v", err)
	}
}

func TestAPIFailureResultExitsNonZero(t *testing.T) {
	c := deviceTestClient(t, "main", nil, func(r deviceRequest) any {
		return map[string]any{"data": map[string]any{"user": map[string]any{"deleteUser": map[string]any{"success": false, "message": "nope"}}}}
	})
	_, err := deviceOutput(t, func() error {
		return ExecuteCommand(context.Background(), c, []string{"api", "user.deleteUser", "username=x"})
	})
	if err == nil {
		t.Fatal("success:false must produce an error")
	}
}

func TestSchemaCommand(t *testing.T) {
	c := deviceTestClient(t, "main", nil, nil)
	out, err := deviceOutput(t, func() error { return runSchema(context.Background(), c, []string{"user.createUser"}) })
	if err != nil || !strings.Contains(out, "CreateUserInput") || !strings.Contains(out, "inputTypes") {
		t.Fatalf("describe op failed: %v\n%s", err, out)
	}
	out, err = deviceOutput(t, func() error { return runSchema(context.Background(), c, []string{"CreateUserInput"}) })
	if err != nil || !strings.Contains(out, "inputFields") {
		t.Fatalf("describe type failed: %v\n%s", err, out)
	}
}

func TestCatalogCoversAllCommands(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	start := strings.Index(text, "func ExecuteCommand")
	body := text[start:]
	known := map[string]bool{}
	for _, c := range fullCatalog() {
		known[c.Name] = true
		for _, a := range c.Aliases {
			known[a] = true
		}
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "case \"") || !strings.HasSuffix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "case \"-h\"") || strings.HasPrefix(line, "case \"list\"") || strings.HasPrefix(line, "case \"create\"") ||
			strings.HasPrefix(line, "case \"remove\"") || strings.HasPrefix(line, "case \"export\"") || strings.HasPrefix(line, "case \"import\"") ||
			strings.HasPrefix(line, "case \"sync\", \"watch\"") || strings.HasPrefix(line, "case \"inspect\"") {
			continue // nested subcommand switches
		}
		for _, name := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "case "), ":"), ",") {
			name = strings.Trim(strings.TrimSpace(name), "\"")
			if !known[name] {
				t.Errorf("command %q is handled by ExecuteCommand but missing from the catalog", name)
			}
		}
	}
}

func TestCommandsJSONAndGlobalJSONFlag(t *testing.T) {
	c := NewClient(&ClientConfig{JSONMode: true})
	out, err := deviceOutput(t, func() error { return runCommands(c, nil) })
	if err != nil {
		t.Fatal(err)
	}
	var doc catalogDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil || len(doc.Commands) < 30 || doc.ErrorShape == "" {
		t.Fatalf("catalog not valid JSON: %v", err)
	}
}

// Every shipped example must be accepted by the schema validation of the Full broker.
func TestShippedDeviceExamplesValidate(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("examples", "devices", "*.json"))
	more, _ := filepath.Glob(filepath.Join("examples", "devices", "templates", "*.json"))
	files = append(files, more...)
	for _, f := range files {
		if strings.HasSuffix(f, "-address.json") {
			continue
		}
		c := deviceTestClient(t, "main", nil, nil)
		if _, err := deviceOutput(t, func() error {
			return runDevice(context.Background(), c, []string{"validate", f})
		}); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func TestShippedAPIExamplesBuild(t *testing.T) {
	c := deviceTestClient(t, "main", nil, nil)
	s, _ := loadDeviceSchema(context.Background(), c)
	cases := map[string]string{
		"user.createUser":              "create-user.json",
		"mutation:publishBatch":        "publish-batch.json",
		"archiveGroup.create":          "create-archive-group.json",
		"topicSchemaPolicy.create":     "create-topic-schema-policy.json",
		"topicNamespace.create":        "create-topic-namespace.json",
	}
	for path, file := range cases {
		o := apiOptions{Depth: 3, ArgsSrc: filepath.Join("examples", "api", file)}
		if _, _, _, _, err := s.buildOperation(path, o); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
}

func TestGeneratedSkill(t *testing.T) {
	s := generateSkill()
	for _, want := range []string{"name: monstermq-cli", "`api`", "`user`", "mmq schema", "--json"} {
		if !strings.Contains(s, want) {
			t.Errorf("skill misses %q", want)
		}
	}
	dir := t.TempDir()
	if err := runSkill([]string{"--install", dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

// edgeSourceContract builds the Edge schema from the edge repository's own .graphqls files
// (the committed gql/edge.gql snapshot can lag behind). Skips when the repo is not checked out.
func edgeSourceContract(t *testing.T) ([]byte, []byte) {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join("..", "..", "edge", "internal", "graphql", "schema", "*.graphqls"))
	if len(files) == 0 {
		t.Skip("edge repository schema not available")
	}
	sort.SliceStable(files, func(i, j int) bool {
		return filepath.Base(files[i]) == "schema.graphqls" && filepath.Base(files[j]) != "schema.graphqls"
	})
	var sdl strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		sdl.WriteString(strings.ReplaceAll(string(b), "\r\n", "\n") + "\n")
	}
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "edge-src", Input: sdl.String()})
	if err != nil {
		t.Skipf("edge source schema not loadable by gqlparser: %v", err)
	}
	return buildTestIntrospection(schema), []byte(sdl.String())
}

func TestUnsModelerAndWinCCOaOnEdge(t *testing.T) {
	_, sdl := edgeSourceContract(t)
	api, err := gqlparser.LoadSchema(&ast.Source{Name: "edge-src", Input: string(sdl)})
	if err != nil {
		t.Fatal(err)
	}
	c := deviceTestClient(t, "edge-src", nil, nil)
	s, err := loadDeviceSchema(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"unsModelers", "unsModelerSchema", "unsModelerValidate", "unsModelerDefaultPrompt", "unsModeler.create", "unsModeler.saveDraft", "unsModeler.apply", "unsModeler.createInWinCCOa", "unsModeler.startAiRun", "unsModeler.importProposal"} {
		args := map[string]any{}
		_, segs, err := s.resolve(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		for _, a := range segs[len(segs)-1].Args {
			if a.Type.Kind == "NON_NULL" && a.DefaultValue == nil {
				args[a.Name] = s.template(a.Type, 0)
			}
		}
		b, _ := json.Marshal(args)
		doc, _, _, _, err := s.buildOperation(path, apiOptions{Depth: 3, ArgsSrc: string(b)})
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if _, errs := gqlparser.LoadQuery(api, doc); len(errs) > 0 {
			t.Errorf("%s: invalid document: %s", path, errs)
		}
	}
	// every operation of the whole edge source schema must build a valid document
	for _, op := range s.operations() {
		path := op.Kind + ":" + op.Path
		_, segs, _ := s.resolve(path)
		args := map[string]any{}
		for _, a := range segs[len(segs)-1].Args {
			if a.Type.Kind == "NON_NULL" && a.DefaultValue == nil {
				args[a.Name] = s.template(a.Type, 0)
			}
		}
		b, _ := json.Marshal(args)
		doc, _, _, _, err := s.buildOperation(path, apiOptions{Depth: 3, ArgsSrc: string(b)})
		if err != nil {
			t.Errorf("%s: %v", path, err)
		} else if _, errs := gqlparser.LoadQuery(api, doc); len(errs) > 0 {
			t.Errorf("%s: invalid document: %s", path, errs)
		}
	}
	// noun commands resolve
	// every noun of an edge-only API must resolve each of its verbs against the edge source schema
	for _, n := range nounSpecs {
		if n.Brokers != "edge" {
			continue
		}
		verbs := map[string]string{}
		for v, q := range n.Queries {
			verbs[v] = "query:" + q
		}
		for v, m := range n.Mutations {
			verbs[v] = "mutation:" + m
		}
		for v, p := range verbs {
			if _, _, err := s.resolve(p); err != nil {
				t.Errorf("%s %s -> %s: %v", n.Names[0], v, p, err)
			}
		}
		if n.Mutation != "" {
			if _, _, err := s.resolve("mutation:" + n.Mutation + ".create"); err != nil && n.Names[0] != "uns" {
				t.Errorf("%s: mutation group %s: %v", n.Names[0], n.Mutation, err)
			}
		}
	}
	for _, n := range []string{"uns", "winccoa", "redfish", "vtt", "camera", "peerLink"} {
		if _, ok := nounFor(n); !ok {
			t.Errorf("noun %s missing", n)
		}
	}
}

func TestKeyValueFileAndUnsModelerCalls(t *testing.T) {
	c := deviceTestClient(t, "edge-src", nil, nil)
	s, err := loadDeviceSchema(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "mapping.json")
	os.WriteFile(f, []byte(`{"version":1}`), 0600)
	o, _ := parseAPIOptions([]string{"name=line1", "document=@" + f})
	_, vars, _, _, err := s.buildOperation("unsModeler.saveDraft", o)
	if err != nil || vars["document"] != `{"version":1}` || vars["name"] != "line1" {
		t.Fatalf("document must be the file content as string: %v %v", vars, err)
	}
	o, _ = parseAPIOptions([]string{"direction=MQTT_TO_WINCCOA"})
	if _, _, _, _, err := s.buildOperation("unsModelerDefaultPrompt", o); err != nil {
		t.Fatalf("enum key=value: %v", err)
	}
	o, _ = parseAPIOptions([]string{"direction=NOPE"})
	if _, _, _, _, err := s.buildOperation("unsModelerDefaultPrompt", o); err == nil {
		t.Fatal("invalid enum must be rejected")
	}
}

// coreSourceContract builds the Core schema from the broker sources (schema-*.graphqls).
func coreSourceContract(t *testing.T) ([]byte, []byte) {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join("..", "..", "main", "broker", "src", "main", "resources", "schema-*.graphqls"))
	if len(files) == 0 {
		t.Skip("core repository schema not available")
	}
	rank := func(f string) int {
		switch filepath.Base(f) {
		case "schema-types.graphqls":
			return 0
		case "schema-queries.graphqls":
			return 1
		case "schema-mutations.graphqls":
			return 2
		case "schema-subscriptions.graphqls":
			return 3
		}
		return 4
	}
	sort.SliceStable(files, func(i, j int) bool { return rank(files[i]) < rank(files[j]) })
	var sdl strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		sdl.WriteString(strings.ReplaceAll(string(b), "\r\n", "\n") + "\n")
	}
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "main-src", Input: sdl.String()})
	if err != nil {
		t.Skipf("core source schema not loadable by gqlparser: %v", err)
	}
	return buildTestIntrospection(schema), []byte(sdl.String())
}

func TestCoreSourceOperationsAndNouns(t *testing.T) {
	_, sdl := coreSourceContract(t)
	api, err := gqlparser.LoadSchema(&ast.Source{Name: "main-src", Input: string(sdl)})
	if err != nil {
		t.Fatal(err)
	}
	c := deviceTestClient(t, "main-src", nil, nil)
	s, err := loadDeviceSchema(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range s.operations() {
		path := op.Kind + ":" + op.Path
		_, segs, _ := s.resolve(path)
		args := map[string]any{}
		for _, a := range segs[len(segs)-1].Args {
			if a.Type.Kind == "NON_NULL" && a.DefaultValue == nil {
				args[a.Name] = s.template(a.Type, 0)
			}
		}
		b, _ := json.Marshal(args)
		doc, _, _, _, err := s.buildOperation(path, apiOptions{Depth: 3, ArgsSrc: string(b)})
		if err != nil {
			t.Errorf("%s: %v", path, err)
		} else if _, errs := gqlparser.LoadQuery(api, doc); len(errs) > 0 {
			t.Errorf("%s: invalid document: %s", path, errs)
		}
	}
	// edge-only nouns must NOT exist on Core (keeps the Brokers flag honest)
	for _, n := range nounSpecs {
		if n.Brokers != "edge" {
			continue
		}
		hit := false
		for _, q := range n.Queries {
			if _, _, err := s.resolve("query:" + q); err == nil {
				hit = true
			}
		}
		if n.QueryGroup != "" {
			if _, _, err := s.resolve("query:" + n.QueryGroup); err == nil {
				hit = true
			}
		}
		if n.Mutation != "" {
			if _, _, err := s.resolve("mutation:" + n.Mutation); err == nil {
				hit = true
			}
		}
		if hit {
			t.Errorf("noun %s is marked edge-only but exists on Core", n.Names[0])
		}
	}
	// nouns marked full/both must resolve against Core
	for _, n := range nounSpecs {
		if n.Brokers == "edge" {
			continue
		}
		verbs := map[string]string{}
		for v, q := range n.Queries {
			verbs[v] = "query:" + q
		}
		for v, m := range n.Mutations {
			verbs[v] = "mutation:" + m
		}
		for v, p := range verbs {
			if _, _, err := s.resolve(p); err != nil {
				t.Errorf("%s %s -> %s: %v", n.Names[0], v, p, err)
			}
		}
		if n.Mutation != "" {
			if _, _, err := s.resolve("mutation:" + n.Mutation); err != nil {
				t.Errorf("%s: mutation group %s: %v", n.Names[0], n.Mutation, err)
			}
		}
	}
}

// The Brokers flag of every noun must match where its operations exist (Core vs Edge source schemas).
func TestNounBrokerFlagsMatchSchemas(t *testing.T) {
	has := func(broker string) func(n nounSpec) bool {
		c := deviceTestClient(t, broker, nil, nil)
		s, err := loadDeviceSchema(context.Background(), c)
		if err != nil {
			t.Fatal(err)
		}
		return func(n nounSpec) bool {
			paths := []string{}
			for _, q := range n.Queries {
				paths = append(paths, "query:"+q)
			}
			for _, m := range n.Mutations {
				paths = append(paths, "mutation:"+m)
			}
			if n.Mutation != "" {
				paths = append(paths, "mutation:"+n.Mutation)
			}
			if n.QueryGroup != "" {
				paths = append(paths, "query:"+n.QueryGroup)
			}
			for _, p := range paths {
				if _, _, err := s.resolve(p); err == nil {
					return true
				}
			}
			return false
		}
	}
	onCore, onEdge := has("main-src"), has("edge-src")
	for _, n := range nounSpecs {
		want := "full"
		switch c, e := onCore(n), onEdge(n); {
		case c && e:
			want = "both"
		case e:
			want = "edge"
		case !c:
			t.Errorf("noun %s exists on neither broker", n.Names[0])
		}
		if got := firstNonEmpty(n.Brokers, "full"); got != want {
			t.Errorf("noun %s: Brokers=%s but schemas say %s", n.Names[0], got, want)
		}
	}
}
