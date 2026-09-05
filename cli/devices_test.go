package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/validator"
)

type deviceRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

var contractOnce sync.Once
var contractFiles map[string][]byte
var contractError error

func testTypeRef(s *ast.Schema, t *ast.Type) map[string]any {
	if t.NonNull {
		copy := *t
		copy.NonNull = false
		return map[string]any{"kind": "NON_NULL", "name": nil, "ofType": testTypeRef(s, &copy)}
	}
	if t.Elem != nil {
		return map[string]any{"kind": "LIST", "name": nil, "ofType": testTypeRef(s, t.Elem)}
	}
	kind := "OBJECT"
	if def, ok := s.Types[t.NamedType]; ok {
		kind = string(def.Kind)
	}
	return map[string]any{"kind": kind, "name": t.NamedType, "ofType": nil}
}

func testInputValue(s *ast.Schema, n, d string, t *ast.Type, v *ast.Value) map[string]any {
	var def any
	if v != nil {
		def = v.String()
	}
	var desc any
	if d != "" {
		desc = d
	}
	return map[string]any{"name": n, "description": desc, "type": testTypeRef(s, t), "defaultValue": def}
}

func buildTestIntrospection(s *ast.Schema) []byte {
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
		item := map[string]any{
			"kind": string(d.Kind), "name": n, "description": d.Description,
			"fields": nil, "inputFields": nil, "enumValues": nil,
		}
		fields, inputs, enums := []any{}, []any{}, []any{}
		for _, f := range d.Fields {
			if strings.HasPrefix(f.Name, "__") {
				continue
			}
			walk(f.Type.Name())
			if d.Kind == ast.InputObject {
				inputs = append(inputs, testInputValue(s, f.Name, f.Description, f.Type, f.DefaultValue))
				continue
			}
			args := []any{}
			for _, a := range f.Arguments {
				args = append(args, testInputValue(s, a.Name, a.Description, a.Type, a.DefaultValue))
				walk(a.Type.Name())
			}
			fields = append(fields, map[string]any{
				"name": f.Name, "description": f.Description, "args": args, "type": testTypeRef(s, f.Type),
			})
		}
		for _, v := range d.EnumValues {
			enums = append(enums, map[string]any{"name": v.Name, "description": v.Description})
		}
		switch d.Kind {
		case ast.Object, ast.Interface:
			item["fields"] = fields
		case ast.InputObject:
			item["inputFields"] = inputs
		case ast.Enum:
			item["enumValues"] = enums
		}
		types = append(types, item)
	}

	if s.Query != nil {
		walk(s.Query.Name)
	}
	if s.Mutation != nil {
		walk(s.Mutation.Name)
	}
	for n := range s.Types {
		walk(n)
	}

	rootName := func(d *ast.Definition) any {
		if d == nil {
			return nil
		}
		return map[string]any{"name": d.Name}
	}

	payload := map[string]any{
		"data": map[string]any{
			"__schema": map[string]any{
				"queryType":        rootName(s.Query),
				"mutationType":     rootName(s.Mutation),
				"subscriptionType": rootName(s.Subscription),
				"types":            types,
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// Read GraphQL schemas directly from the gql directory or MMQ_CONTRACT_DIR.
func deviceContract(t *testing.T, broker string) ([]byte, []byte) {
	t.Helper()
	contractOnce.Do(func() {
		contractFiles = map[string][]byte{}
		dir := os.Getenv("MMQ_CONTRACT_DIR")
		if dir != "" {
			for _, name := range []string{"main", "edge"} {
				for _, file := range []string{"introspection.json", "schema.graphql"} {
					data, err := os.ReadFile(filepath.Join(dir, name, file))
					if err != nil {
						contractError = err
						return
					}
					contractFiles[name+"/"+file] = data
				}
			}
			return
		}

		// Locate the gql directory containing main.gql and edge.gql
		gqlDir := ""
		for _, cand := range []string{"../gql", "gql", "../../gql"} {
			if _, err := os.Stat(filepath.Join(cand, "main.gql")); err == nil {
				gqlDir = cand
				break
			}
		}
		if gqlDir == "" {
			contractError = fmt.Errorf("cannot find gql directory containing main.gql and edge.gql")
			return
		}

		for _, name := range []string{"main", "edge"} {
			sdlBytes, err := os.ReadFile(filepath.Join(gqlDir, name+".gql"))
			if err != nil {
				contractError = fmt.Errorf("failed to read %s.gql: %w", name, err)
				return
			}
			schema, err := gqlparser.LoadSchema(&ast.Source{Name: name, Input: string(sdlBytes)})
			if err != nil {
				contractError = fmt.Errorf("failed to parse %s.gql schema: %w", name, err)
				return
			}
			introJSON := buildTestIntrospection(schema)
			contractFiles[name+"/schema.graphql"] = sdlBytes
			contractFiles[name+"/introspection.json"] = introJSON
		}
	})
	if contractError != nil {
		t.Fatal(contractError)
	}
	if broker == "full" {
		broker = "main"
	}
	return contractFiles[broker+"/introspection.json"], contractFiles[broker+"/schema.graphql"]
}

func deviceTestClient(t *testing.T, broker string, existing map[string]any, handle func(deviceRequest) any) *Client {
	t.Helper()
	schema, sdl := deviceContract(t, broker)
	api, e := gqlparser.LoadSchema(&ast.Source{Name: broker, Input: string(sdl)})
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req deviceRequest
		if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
			t.Error(e)
			return
		}
		operation, errs := gqlparser.LoadQuery(api, req.Query)
		if len(errs) > 0 {
			t.Errorf("invalid GraphQL request: %s", errs)
			http.Error(w, "invalid query", 400)
			return
		}
		if _, err := validator.VariableValues(api, operation.Operations[0], req.Variables); err != nil {
			t.Errorf("invalid GraphQL variables: %s", err)
			http.Error(w, "invalid variables", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Query, "__schema") {
			w.Write(schema)
			return
		}
		if strings.Contains(req.Query, "getDevices") {
			items := []any{}
			if existing != nil {
				items = append(items, existing)
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"getDevices": items}})
			return
		}
		if handle != nil {
			json.NewEncoder(w).Encode(handle(req))
			return
		}
		t.Errorf("unexpected request: %s", req.Query)
		json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"message": "unexpected request"}}})
	}))
	t.Cleanup(server.Close)
	return NewClient(&ClientConfig{URL: server.URL, JSONMode: true})
}
func deviceTestFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "input.json")
	if e := os.WriteFile(p, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func deviceOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	file, e := os.CreateTemp(t.TempDir(), "output")
	if e != nil {
		t.Fatal(e)
	}
	old := os.Stdout
	os.Stdout = file
	defer func() { os.Stdout = old; file.Close() }()
	err := fn()
	file.Seek(0, 0)
	b, _ := io.ReadAll(file)
	return string(b), err
}
func deviceRun(t *testing.T, c *Client, args ...string) (string, error) {
	return deviceOutput(t, func() error { return ExecuteCommand(context.Background(), c, append([]string{"device"}, args...)) })
}
func deviceStored(typ string) map[string]any {
	return map[string]any{"name": "test", "namespace": "plant/test", "nodeId": "node1", "type": typ, "enabled": true, "config": map[string]any{"brokerUrl": "tcp://localhost:1883", "password": "original-password", "keepAlive": float64(60), "addresses": []any{map[string]any{"remoteTopic": "a/#"}}}}
}
func TestDeviceEnableUsesNativeToggle(t *testing.T) {
	for _, typ := range []string{"MQTT-Client", "OPCUA-Client"} {
		t.Run(typ, func(t *testing.T) {
			a, _ := adapterFor(typ)
			mutations := 0
			c := deviceTestClient(t, "full", deviceStored(typ), func(r deviceRequest) any {
				mutations++
				if !strings.Contains(r.Query, a.Root+" { toggle(") || strings.Contains(r.Query, "importDevices") {
					t.Errorf("wrong toggle: %s", r.Query)
				}
				if r.Variables["enabled"] != true {
					t.Error("not enabling")
				}
				return map[string]any{"data": map[string]any{a.Root: map[string]any{"toggle": map[string]any{"success": true, "errors": []any{}}}}}
			})
			_, e := deviceRun(t, c, "enable", "test")
			if e != nil {
				t.Fatal(e)
			}
			if mutations != 1 {
				t.Fatalf("%d mutations", mutations)
			}
		})
	}
}
func TestDeviceMutationFailures(t *testing.T) {
	for _, response := range []string{`{"errors":[{"message":"denied"}]}`, `{"data":{"mqttClient":{"toggle":{"success":false,"errors":["failed"]}}}}`, `{"data":{"mqttClient":null}}`, `{"data":{"mqttClient":{"toggle":null}}}`} {
		c := deviceTestClient(t, "full", deviceStored("MQTT-Client"), func(r deviceRequest) any { var v any; json.Unmarshal([]byte(response), &v); return v })
		out, e := deviceRun(t, c, "enable", "test")
		if e == nil || !strings.Contains(out, `"success": false`) {
			t.Fatalf("failure not propagated: %s %v", out, e)
		}
	}
}
func TestDeviceApplyPreservesOmittedFields(t *testing.T) {
	c := deviceTestClient(t, "full", deviceStored("MQTT-Client"), func(r deviceRequest) any {
		if !strings.Contains(r.Query, "update(") {
			t.Errorf("expected update: %s", r.Query)
		}
		input := r.Variables["input"].(map[string]any)
		cfg := input["config"].(map[string]any)
		if cfg["password"] != "original-password" || cfg["brokerUrl"] != "tcp://localhost:1883" || cfg["keepAlive"] != float64(15) || input["enabled"] != true {
			t.Errorf("bad merge: %#v", input)
		}
		if _, ok := cfg["addresses"]; ok {
			t.Error("addresses must not enter connection mutation")
		}
		return map[string]any{"data": map[string]any{"mqttClient": map[string]any{"update": map[string]any{"success": true}}}}
	})
	p := deviceTestFile(t, `{"type":"mqtt","input":{"name":"test","config":{"keepAlive":15}}}`)
	out, e := deviceRun(t, c, "apply", p)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(out, "original-password") {
		t.Fatal("leaked credential")
	}
}
func TestDeviceCreateDefaultsDisabled(t *testing.T) {
	c := deviceTestClient(t, "full", nil, func(r deviceRequest) any {
		if !strings.Contains(r.Query, "add(") {
			t.Errorf("expected OPC UA add: %s", r.Query)
		}
		input := r.Variables["input"].(map[string]any)
		if input["enabled"] != false {
			t.Error("new device must default to disabled")
		}
		return map[string]any{"data": map[string]any{"opcUaDevice": map[string]any{"add": map[string]any{"success": true}}}}
	})
	p := deviceTestFile(t, `{"type":"opcua","input":{"name":"test","namespace":"plant/test","nodeId":"node1","config":{"endpointUrl":"opc.tcp://localhost:4840"}}}`)
	if _, e := deviceRun(t, c, "apply", p); e != nil {
		t.Fatal(e)
	}
}
func TestDeviceValidationAndDryRunNeverMutate(t *testing.T) {
	c := deviceTestClient(t, "full", deviceStored("MQTT-Client"), nil)
	for _, action := range []string{"apply", "validate"} {
		args := []string{action, deviceTestFile(t, `{"type":"mqtt","input":{"name":"test","config":{"keepAlive":10}}}`)}
		if action == "apply" {
			args = append(args, "--dry-run")
		}
		out, e := deviceRun(t, c, args...)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(out, "original-password") || !strings.Contains(out, "[REDACTED]") {
			t.Fatalf("unsafe output: %s", out)
		}
	}
	for _, body := range []string{`{"type":"mqtt","input":{"name":"test","config":{"keepAliv":10}}}`, `{"type":"mqtt","input":{"name":"test","config":{"keepAlive":"10"}}}`, `{"type":"mqtt","input":{"name":"test","config":{"brokerUrl":null}}}`, `{"type":"mqtt","input":{"name":"test","config":{"addresses":[]}}}`} {
		if _, e := deviceRun(t, c, "apply", deviceTestFile(t, body)); e == nil {
			t.Fatalf("accepted invalid input %s", body)
		}
	}
}
func TestDeviceEdgeRejectsOPCUA(t *testing.T) {
	c := deviceTestClient(t, "edge", deviceStored("OPCUA-Client"), nil)
	for _, args := range [][]string{{"schema", "opcua"}, {"enable", "test"}} {
		_, e := deviceRun(t, c, args...)
		if e == nil || !strings.Contains(e.Error(), "unavailable") {
			t.Fatalf("unexpected result: %v", e)
		}
	}
}
func TestDeviceAddressMutation(t *testing.T) {
	c := deviceTestClient(t, "edge", deviceStored("MQTT-Client"), func(r deviceRequest) any {
		if !strings.Contains(r.Query, "addAddress(") || r.Variables["deviceName"] != "test" {
			t.Errorf("bad request %#v", r)
		}
		return map[string]any{"data": map[string]any{"mqttClient": map[string]any{"addAddress": map[string]any{"success": true}}}}
	})
	p := deviceTestFile(t, `{"mode":"SUBSCRIBE","remoteTopic":"remote/#","localTopic":"local/#","qos":1}`)
	if _, e := deviceRun(t, c, "address", "add", "test", p); e != nil {
		t.Fatal(e)
	}
}
func TestDeviceImportPartialFailure(t *testing.T) {
	c := deviceTestClient(t, "full", nil, func(r deviceRequest) any {
		return map[string]any{"data": map[string]any{"importDevices": map[string]any{"success": true, "imported": 1, "failed": 1, "total": 2, "errors": []any{"duplicate"}}}}
	})
	p := deviceTestFile(t, `[{"name":"one","namespace":"one","nodeId":"*"},{"name":"two","namespace":"two","nodeId":"*"}]`)
	out, e := deviceRun(t, c, "upload", p)
	if e == nil || !strings.Contains(out, "1/2 imported") {
		t.Fatalf("partial failure: %s %v", out, e)
	}
}
func TestDeviceWaitUnsupportedBeforeMutation(t *testing.T) {
	c := deviceTestClient(t, "full", deviceStored("MQTT-Client"), nil)
	if _, e := deviceRun(t, c, "enable", "test", "--wait"); e == nil || !strings.Contains(e.Error(), "does not expose") {
		t.Fatalf("%v", e)
	}
}
func TestDeviceWaitTimeout(t *testing.T) {
	c := deviceTestClient(t, "full", deviceStored("OPCUA-Client"), func(r deviceRequest) any {
		return map[string]any{"data": map[string]any{"opcuaServers": []any{map[string]any{"name": "test", "enabled": true, "connected": false}}}}
	})
	start := time.Now()
	_, e := deviceRun(t, c, "status", "test", "--wait", "--timeout", "10ms")
	if e == nil || !strings.Contains(e.Error(), "timed out") {
		t.Fatalf("%v", e)
	}
	if time.Since(start) > time.Second {
		t.Error("timeout ignored")
	}
}
func TestDeviceOptionParsingAndHelp(t *testing.T) {
	c := NewClient(&ClientConfig{URL: "http://invalid", JSONMode: true})
	for _, args := range [][]string{{"apply"}, {"enable"}, {"apply", "x", "--invalid"}, {"enable", "x", "--timeout", "0s"}, {"get", "x", "--dry-run"}, {"status", "x", "--timeout"}} {
		if _, e := deviceRun(t, c, args...); e == nil {
			t.Errorf("accepted %#v", args)
		}
	}
	if _, e := deviceRun(t, c, "apply", "--help"); e != nil {
		t.Fatal(e)
	}
}
func TestDeviceSchemaAndTemplate(t *testing.T) {
	for _, broker := range []string{"full", "edge"} {
		c := deviceTestClient(t, broker, nil, nil)
		out, e := deviceRun(t, c, "schema", "mqtt")
		if e != nil || !strings.Contains(out, "brokerUrl") {
			t.Fatalf("%s %v", out, e)
		}
		out, e = deviceRun(t, c, "template", "mqtt")
		if e != nil {
			t.Fatal(e)
		}
		var doc map[string]any
		if e = json.Unmarshal([]byte(out), &doc); e != nil {
			t.Fatal(e)
		}
		if doc["input"].(map[string]any)["enabled"] != false {
			t.Error("unsafe template")
		}
	}
}
func TestDeviceStdin(t *testing.T) {
	p := deviceTestFile(t, `{"hello":true}`)
	f, _ := os.Open(p)
	defer f.Close()
	old := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = old }()
	v, e := readDeviceJSON("-")
	if e != nil || v["hello"] != true {
		t.Fatalf("%v %v", v, e)
	}
}

func TestDeviceAllAdapterContracts(t *testing.T) {
	c := deviceTestClient(t, "full", nil, nil)
	s, e := loadDeviceSchema(context.Background(), c)
	if e != nil {
		t.Fatal(e)
	}
	for _, a := range deviceAdapters {
		t.Run(a.Type, func(t *testing.T) {
			for _, op := range []string{a.Create, "update", "toggle", "delete"} {
				if _, e := s.operation(a.Root, op); e != nil {
					t.Error(e)
				}
			}
			f, e := s.operation(a.Root, a.Create)
			if e != nil {
				return
			}
			arg, ok := fieldNamed(f.Args, "input")
			if !ok {
				t.Fatal("missing input")
			}
			b, _ := json.Marshal(s.template(arg.Type, 0))
			var v any
			json.Unmarshal(b, &v)
			if e := s.validate(arg.Type, v, "input"); e != nil {
				t.Error(e)
			}
			if _, ok := fieldNamed(s.Types[s.Query].Fields, a.Query); !ok {
				t.Errorf("missing query %s", a.Query)
			}
		})
	}
}
func TestDeviceServerUpdatePreservesConfig(t *testing.T) {
	stored := map[string]any{"name": "test", "type": "Kafka-Server", "namespace": "kafka/test", "nodeId": "node1", "enabled": true, "config": map[string]any{"port": float64(9093), "streams": []any{}, "host": "127.0.0.1"}}
	c := deviceTestClient(t, "full", stored, func(r deviceRequest) any {
		input := r.Variables["input"].(map[string]any)
		if input["port"] != float64(9093) || input["host"] != "127.0.0.1" {
			t.Error("server config was reset")
		}
		return map[string]any{"data": map[string]any{"kafkaServer": map[string]any{"update": map[string]any{"success": true}}}}
	})
	p := deviceTestFile(t, `{"type":"Kafka-Server","input":{"name":"test","enabled":false}}`)
	if _, e := deviceRun(t, c, "apply", p); e != nil {
		t.Fatal(e)
	}
}
func TestDeviceOPCUAAddressUpdateUnsupported(t *testing.T) {
	c := deviceTestClient(t, "full", deviceStored("OPCUA-Client"), nil)
	_, e := deviceRun(t, c, "address", "update", "test", "ns=2;i=1", "unused.json")
	if e == nil || !strings.Contains(e.Error(), "unavailable") {
		t.Fatalf("%v", e)
	}
}
func TestDeviceDeleteFalse(t *testing.T) {
	c := deviceTestClient(t, "full", deviceStored("MQTT-Client"), func(r deviceRequest) any {
		return map[string]any{"data": map[string]any{"mqttClient": map[string]any{"delete": false}}}
	})
	if _, e := deviceRun(t, c, "delete", "test"); e == nil {
		t.Fatal("false delete accepted")
	}
}
func TestDeviceCallReassignAndDryRun(t *testing.T) {
	calls := 0
	c := deviceTestClient(t, "full", nil, func(r deviceRequest) any {
		calls++
		if r.Variables["nodeId"] != "node2" {
			t.Error("missing assignment")
		}
		return map[string]any{"data": map[string]any{"mqttClient": map[string]any{"reassign": map[string]any{"success": true}}}}
	})
	p := deviceTestFile(t, `{"name":"test","nodeId":"node2"}`)
	if _, e := deviceRun(t, c, "call", "mqtt", "reassign", p, "--dry-run"); e != nil {
		t.Fatal(e)
	}
	if calls != 0 {
		t.Fatal("dry-run mutated")
	}
	if _, e := deviceRun(t, c, "call", "mqtt", "reassign", p); e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatal("call did not execute")
	}
}
func TestDeviceBrowseAndStatus(t *testing.T) {
	for _, action := range []string{"browse", "status"} {
		c := deviceTestClient(t, "full", deviceStored("OPCUA-Client"), func(r deviceRequest) any {
			if action == "browse" {
				if r.Variables["serverId"] != "test" || r.Variables["nodeId"] != "i=85" {
					t.Error("incorrect browsing arguments")
				}
				return map[string]any{"data": map[string]any{"opcuaNodeBrowse": []any{}}}
			}
			return map[string]any{"data": map[string]any{"opcuaServers": []any{map[string]any{"name": "test", "connected": true, "enabled": true}}}}
		})
		out, e := deviceRun(t, c, action, "test")
		if e != nil {
			t.Fatal(e)
		}
		if action == "status" && !strings.Contains(out, `"runtimeKnown": true`) {
			t.Error(out)
		}
	}
}

func TestSessionRemovalMatchesBrokerContract(t *testing.T) {
	c := deviceTestClient(t, "full", nil, func(r deviceRequest) any {
		if !strings.Contains(r.Query, "session {") || !strings.Contains(r.Query, "results {") {
			t.Errorf("wrong session contract: %s", r.Query)
		}
		return map[string]any{"data": map[string]any{"session": map[string]any{"removeSessions": map[string]any{"results": []any{map[string]any{"clientId": "test-client", "success": true}}}}}}
	})
	out, err := deviceOutput(t, func() error {
		return ExecuteCommand(context.Background(), c, []string{"session", "remove", "test-client"})
	})
	if err != nil || !strings.Contains(out, "test-client") {
		t.Fatalf("%s %v", out, err)
	}
}
