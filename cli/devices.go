package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

type deviceAdapter struct{ Type, Root, Query, Create string }

var deviceAdapters = []deviceAdapter{
	{"OPCUA-Client", "opcUaDevice", "opcUaDevices", "add"},
	{"MQTT-Client", "mqttClient", "mqttClients", "create"},
	{"KAFKA-Client", "kafkaClient", "kafkaClients", "create"},
	{"WinCCOA-Client", "winCCOaDevice", "winCCOaClients", "create"},
	{"WinCCUA-Client", "winCCUaDevice", "winCCUaClients", "create"},
	{"PLC4X-Client", "plc4xDevice", "plc4xClients", "create"},
	{"NATS-Client", "natsClient", "natsClients", "create"},
	{"Redis-Client", "redisClient", "redisClients", "create"},
	{"Neo4j-Client", "neo4jClient", "neo4jClients", "create"},
	{"Telegram-Client", "telegramClient", "telegramClients", "create"},
	{"I3X-Client", "i3xClient", "i3xClients", "create"},
	{"OPCUA-Server", "opcUaServer", "opcUaServers", "add"},
	{"Kafka-Server", "kafkaServer", "kafkaServers", "add"},
	{"JDBC-Logger", "jdbcLogger", "jdbcLoggers", "create"},
	{"InfluxDB-Logger", "influxdbLogger", "influxdbLoggers", "create"},
	{"TimeBase-Logger", "timebaseLogger", "timebaseLoggers", "create"},
	{"SparkplugB-Decoder", "sparkplugBDecoder", "sparkplugBDecoders", "create"},
}

func normalizedDeviceType(v string) string {
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(v))
}
func adapterFor(v string) (deviceAdapter, error) {
	n := normalizedDeviceType(v)
	for _, a := range deviceAdapters {
		if n == normalizedDeviceType(a.Type) || n == normalizedDeviceType(a.Root) || n == strings.TrimSuffix(normalizedDeviceType(a.Type), "client") {
			return a, nil
		}
	}
	return deviceAdapter{}, fmt.Errorf("unsupported device type %q; use device types", v)
}

const deviceHelp = `Usage: mmq [global options] device <command>
  list [type]                       List configured devices
  types                             Discover API operations and enabled broker features
  schema <type> [operation]          Describe mutation arguments and nested input types
  template <type>                    Generate an editable {type,input} JSON document
  get <name>                        Read configuration (secrets redacted)
  download [name] [file.json]        Export raw configuration, including credentials
  upload <file.json|->              Backup import (devices are imported disabled)
  validate <file.json|->             Validate an apply document against this broker
  apply <file.json|-> [--dry-run] [--wait] [--timeout 30s]
                                    Merge supplied fields and create/update a device
  enable|disable <name> [--wait] [--timeout 30s]
  delete <name>                      Delete through the device API
  status <name> [--wait] [--timeout 30s]
                                    Read state; --wait requires runtime connectivity
  browse <name> [nodeId]             Browse an OPC UA client (default i=85)
  address list <name>                Read configured mappings
  address add <name> <file.json|->   Add a mapping (native input object)
  address update <name> <key> <file.json|->
  address delete <name> <key>        Remove a mapping
  call <type> <operation> <args.json|-> [--dry-run]
                                    Invoke a discovered native mutation (e.g. reassign)

Use --json before device for machine-readable output. New devices default to disabled.
Apply input is a partial update: omitted fields/credentials are preserved, arrays replace.
MQTT/OPC UA addresses use separate address commands. No implicit deletion of mappings.
Schema validation does not test connectivity or replace broker-side semantic validation.
`

type deviceOptions struct {
	Args         []string
	DryRun, Wait bool
	Timeout      time.Duration
}

func parseDeviceOptions(args []string) (deviceOptions, error) {
	o := deviceOptions{Timeout: 30 * time.Second}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			o.DryRun = true
		case "--wait":
			o.Wait = true
		case "--timeout":
			i++
			if i == len(args) {
				return o, fmt.Errorf("--timeout requires a duration")
			}
			d, e := time.ParseDuration(args[i])
			if e != nil || d <= 0 {
				return o, fmt.Errorf("--timeout must be a positive duration")
			}
			o.Timeout = d
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				return o, fmt.Errorf("unknown device option %s", args[i])
			}
			o.Args = append(o.Args, args[i])
		}
	}
	return o, nil
}
func readDeviceJSON(path string) (map[string]any, error) {
	var r io.Reader = os.Stdin
	if path != "-" {
		f, e := os.Open(path)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		r = f
	}
	dec := json.NewDecoder(r)
	var obj map[string]any
	if e := dec.Decode(&obj); e != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", e)
	}
	if obj == nil {
		return nil, fmt.Errorf("expected JSON object")
	}
	var extra any
	if e := dec.Decode(&extra); e != io.EOF {
		return nil, fmt.Errorf("expected a single JSON object")
	}
	if hasRedactedValue(obj) {
		return nil, fmt.Errorf("input contains [REDACTED]; omit these fields to preserve stored credentials")
	}
	return obj, nil
}
func hasRedactedValue(v any) bool {
	switch x := v.(type) {
	case string:
		return x == "[REDACTED]"
	case map[string]any:
		for _, item := range x {
			if hasRedactedValue(item) {
				return true
			}
		}
	case []any:
		for _, item := range x {
			if hasRedactedValue(item) {
				return true
			}
		}
	}
	return false
}
func redactDevice(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			low := strings.ToLower(k)
			if strings.Contains(low, "password") || strings.Contains(low, "secret") || strings.Contains(low, "token") || strings.Contains(low, "privatekey") {
				if val != nil {
					out[k] = "[REDACTED]"
				} else {
					out[k] = nil
				}
			} else {
				out[k] = redactDevice(val)
			}
		}
		return out
	case []any:
		out := []any{}
		for _, val := range x {
			out = append(out, redactDevice(val))
		}
		return out
	default:
		return v
	}
}
func mergeDevice(base, patch map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		a, aok := out[k].(map[string]any)
		b, bok := v.(map[string]any)
		if aok && bok {
			out[k] = mergeDevice(a, b)
		} else {
			out[k] = v
		}
	}
	return out
}
func getDevice(ctx context.Context, c *Client, name string) (map[string]any, error) {
	data, e := deviceQuery(ctx, c, `query DeviceGet($names: [String!]) { getDevices(names: $names) { name namespace nodeId type enabled config } }`, map[string]any{"names": []string{name}})
	if e != nil {
		return nil, e
	}
	items, ok := data["getDevices"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid getDevices response")
	}
	for _, item := range items {
		if d, ok := item.(map[string]any); ok && d["name"] == name {
			return d, nil
		}
	}
	return nil, nil
}
func requireDevice(ctx context.Context, c *Client, name string) (map[string]any, deviceAdapter, error) {
	d, e := getDevice(ctx, c, name)
	if e != nil {
		return nil, deviceAdapter{}, e
	}
	if d == nil {
		return nil, deviceAdapter{}, fmt.Errorf("device %q not found (check DeviceImportExport feature)", name)
	}
	a, e := adapterFor(fmt.Sprint(d["type"]))
	return d, a, e
}
func (s *deviceSchema) invoke(ctx context.Context, c *Client, a deviceAdapter, op string, args map[string]any, dry bool) (any, error) {
	f, e := s.operation(a.Root, op)
	if e != nil {
		return nil, e
	}
	for key := range args {
		if _, ok := fieldNamed(f.Args, key); !ok {
			return nil, fmt.Errorf("%s.%s: unknown argument %s", a.Root, op, key)
		}
	}
	defs, uses := []string{}, []string{}
	for _, arg := range f.Args {
		v, ok := args[arg.Name]
		if !ok && arg.DefaultValue != nil {
			continue
		}
		if e := s.validate(arg.Type, v, arg.Name); e != nil {
			return nil, e
		}
		if ok {
			defs = append(defs, "$"+arg.Name+": "+arg.Type.String())
			uses = append(uses, arg.Name+": $"+arg.Name)
		}
	}
	if dry {
		return map[string]any{"operation": a.Root + "." + op, "arguments": redactDevice(args), "dryRun": true}, nil
	}
	selection := ""
	t := s.Types[f.Type.base()]
	if t.Kind == "OBJECT" {
		parts := []string{}
		for _, n := range []string{"success", "errors", "message"} {
			if _, ok := fieldNamed(t.Fields, n); ok {
				parts = append(parts, n)
			}
		}
		if len(parts) == 0 {
			parts = append(parts, "__typename")
		}
		selection = " { " + strings.Join(parts, " ") + " }"
	}
	def, use := "", ""
	if len(defs) > 0 {
		def = "(" + strings.Join(defs, ", ") + ")"
		use = "(" + strings.Join(uses, ", ") + ")"
	}
	data, e := deviceQuery(ctx, c, "mutation DeviceOperation"+def+" { "+a.Root+" { "+op+use+selection+" } }", args)
	if e != nil {
		return nil, e
	}
	root, ok := data[a.Root].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s returned no result", a.Root)
	}
	result := root[op]
	if result == nil {
		return nil, fmt.Errorf("%s.%s returned no result", a.Root, op)
	}
	if b, ok := result.(bool); ok && !b {
		return nil, fmt.Errorf("%s.%s failed", a.Root, op)
	}
	if obj, ok := result.(map[string]any); ok {
		_, expectsSuccess := fieldNamed(t.Fields, "success")
		failed := expectsSuccess && obj["success"] != true
		if es, ok := obj["errors"].([]any); ok && len(es) > 0 {
			failed = true
		}
		if failed {
			b, _ := json.Marshal(obj)
			return nil, fmt.Errorf("%s.%s failed: %s", a.Root, op, b)
		}
	}
	return result, nil
}
func runDevice(ctx context.Context, c *Client, args []string) (err error) {
	defer func() {
		if err != nil && c.cfg.JSONMode {
			_ = printJSON(map[string]any{"success": false, "error": err.Error()})
		}
	}()
	if hasHelpFlag(args) || (len(args) > 0 && args[0] == "help") {
		fmt.Print(deviceHelp)
		return nil
	}
	if len(args) == 0 {
		return runDeviceList(ctx, c, nil)
	}
	action := args[0]
	rest := args[1:]
	switch action {
	case "list":
		return runDeviceList(ctx, c, rest)
	case "download":
		return runDeviceDownload(ctx, c, rest)
	case "upload":
		return runDeviceUpload(ctx, c, rest)
	}
	o, e := parseDeviceOptions(rest)
	if e != nil {
		return e
	}
	for _, token := range rest {
		if token == "--timeout" && !o.Wait {
			return fmt.Errorf("--timeout requires --wait")
		}
	}
	if o.Wait && o.DryRun {
		return fmt.Errorf("--wait cannot be combined with --dry-run")
	}
	p := o.Args
	if o.DryRun && action != "apply" && action != "call" {
		return fmt.Errorf("--dry-run is only supported by apply and call")
	}
	if o.Wait && action != "apply" && action != "enable" && action != "disable" && action != "status" {
		return fmt.Errorf("--wait is not supported by %s", action)
	}
	counts := map[string][2]int{"types": {0, 0}, "schema": {1, 2}, "template": {1, 1}, "get": {1, 1}, "validate": {1, 1}, "apply": {1, 1}, "enable": {1, 1}, "disable": {1, 1}, "delete": {1, 1}, "status": {1, 1}, "browse": {1, 2}, "address": {2, 4}, "call": {3, 3}}
	count, ok := counts[action]
	if !ok {
		return fmt.Errorf("unknown device action %q; use device --help", action)
	}
	if len(p) < count[0] || len(p) > count[1] {
		return fmt.Errorf("invalid arguments for device %s; use device --help", action)
	}
	if action == "get" {
		d, e := getDevice(ctx, c, p[0])
		if e != nil {
			return e
		}
		if d == nil {
			return fmt.Errorf("device %q not found", p[0])
		}
		return printJSON(redactDevice(d))
	}
	s, e := loadDeviceSchema(ctx, c)
	if e != nil {
		return e
	}
	switch action {
	case "types":
		items := []any{}
		for _, a := range deviceAdapters {
			root, ok := fieldNamed(s.Types[s.Mutation].Fields, a.Root)
			if !ok {
				continue
			}
			ops := []string{}
			for _, f := range s.Types[root.Type.base()].Fields {
				ops = append(ops, f.Name)
			}
			sort.Strings(ops)
			items = append(items, map[string]any{"type": a.Type, "api": a.Root, "operations": ops})
		}
		features, e := deviceQuery(ctx, c, `query { broker { enabledFeatures } }`, nil)
		if e != nil {
			return e
		}
		return printJSON(map[string]any{"types": items, "broker": features["broker"], "note": "API availability is not feature activation; the broker enforces feature and node permissions."})
	case "schema", "template":
		a, e := adapterFor(p[0])
		if e != nil {
			return e
		}
		op := a.Create
		if len(p) > 1 {
			op = p[1]
		}
		f, e := s.operation(a.Root, op)
		if e != nil {
			return e
		}
		if action == "schema" {
			ts := map[string]gqlType{}
			for _, arg := range f.Args {
				s.inputTypes(arg.Type, ts)
			}
			return printJSON(map[string]any{"type": a.Type, "operation": op, "arguments": f.Args, "types": ts})
		}
		input, ok := fieldNamed(f.Args, "input")
		if !ok {
			return fmt.Errorf("%s requires native arguments; use device schema and device call", a.Type)
		}
		obj, ok := s.template(input.Type, 0).(map[string]any)
		if !ok {
			return fmt.Errorf("invalid input schema")
		}
		obj["name"] = "my-device"
		if _, ok := obj["namespace"]; ok {
			obj["namespace"] = "devices/my-device"
		}
		if _, ok := obj["nodeId"]; ok {
			obj["nodeId"] = "*"
		}
		if _, ok := fieldNamed(s.Types[input.Type.base()].InputFields, "enabled"); ok {
			obj["enabled"] = false
		}
		return printJSON(map[string]any{"type": a.Type, "input": obj})
	case "validate", "apply":
		return applyDevice(ctx, c, s, p[0], o, action == "validate")
	case "call":
		a, e := adapterFor(p[0])
		if e != nil {
			return e
		}
		vars, e := readDeviceJSON(p[2])
		if e != nil {
			return e
		}
		res, e := s.invoke(ctx, c, a, p[1], vars, o.DryRun)
		if e != nil {
			return e
		}
		return printJSON(res)
	}
	deviceName := p[0]
	if action == "address" {
		deviceName = p[1]
	}
	d, a, e := requireDevice(ctx, c, deviceName)
	if e != nil {
		return e
	}
	name := fmt.Sprint(d["name"])
	switch action {
	case "enable", "disable", "delete":
		op := "toggle"
		vars := map[string]any{"name": name}
		if action == "delete" {
			op = "delete"
			f, e := s.operation(a.Root, op)
			if e != nil {
				return e
			}
			if _, ok := fieldNamed(f.Args, "serverName"); ok {
				vars = map[string]any{"serverName": name}
			}
		} else {
			vars["enabled"] = action == "enable"
		}
		if o.Wait {
			if e := s.canWait(a); e != nil {
				return e
			}
		}
		res, e := s.invoke(ctx, c, a, op, vars, false)
		if e != nil {
			return e
		}
		if o.Wait {
			return waitDevice(ctx, c, s, a, name, action == "enable", o.Timeout)
		}
		return printJSON(map[string]any{"success": true, "device": name, "result": res})
	case "status":
		if o.Wait {
			return waitDevice(ctx, c, s, a, name, true, o.Timeout)
		}
		res, e := s.deviceStatus(ctx, c, a, name)
		if e != nil {
			return e
		}
		return printJSON(res)
	case "browse":
		if a.Root != "opcUaDevice" {
			return fmt.Errorf("browse is currently supported for OPC UA clients only")
		}
		if _, ok := fieldNamed(s.Types[s.Query].Fields, "opcuaNodeBrowse"); !ok {
			return fmt.Errorf("OPC UA browsing is unavailable on this broker")
		}
		node := "i=85"
		if len(p) > 1 {
			node = p[1]
		}
		data, e := deviceQuery(ctx, c, `query DeviceBrowse($serverId: String!, $nodeId: String!) { opcuaNodeBrowse(serverId: $serverId, nodeId: $nodeId) { nodeId browseName displayName nodeClass dataType hasChildren writable } }`, map[string]any{"serverId": name, "nodeId": node})
		if e != nil {
			return e
		}
		return printJSON(data["opcuaNodeBrowse"])
	case "address":
		return deviceAddress(ctx, c, s, a, d, p)
	}
	return fmt.Errorf("unsupported command")
}

func applyDevice(ctx context.Context, c *Client, s *deviceSchema, path string, o deviceOptions, validateOnly bool) error {
	doc, e := readDeviceJSON(path)
	if e != nil {
		return e
	}
	for key := range doc {
		if key != "type" && key != "input" {
			return fmt.Errorf("unknown document field %s; expected {type,input}; use upload for raw backups", key)
		}
	}
	typ, ok := doc["type"].(string)
	if !ok {
		return fmt.Errorf("type is required")
	}
	a, e := adapterFor(typ)
	if e != nil {
		return e
	}
	patch, ok := doc["input"].(map[string]any)
	if !ok {
		return fmt.Errorf("input must be an object")
	}
	name, ok := patch["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("input.name is required")
	}
	existing, e := getDevice(ctx, c, name)
	if e != nil {
		return e
	}
	op := a.Create
	if existing != nil {
		old, e := adapterFor(fmt.Sprint(existing["type"]))
		if e != nil {
			return e
		}
		if old.Type != a.Type {
			return fmt.Errorf("cannot change device %s from %s to %s", name, old.Type, a.Type)
		}
		op = "update"
	}
	f, e := s.operation(a.Root, op)
	if e != nil {
		return e
	}
	arg, ok := fieldNamed(f.Args, "input")
	if !ok {
		return fmt.Errorf("%s.%s requires native arguments; use device call", a.Root, op)
	}
	base := map[string]any{}
	if existing != nil {
		stored := existing
		// Server APIs expose stored config fields at the input's top level.
		if _, hasConfig := fieldNamed(s.Types[arg.Type.base()].InputFields, "config"); !hasConfig {
			if cfg, ok := existing["config"].(map[string]any); ok {
				stored = mergeDevice(cfg, existing)
			}
		}
		base, _ = s.project(arg.Type, stored).(map[string]any)
	}
	input := mergeDevice(base, patch)
	if existing == nil {
		if _, ok := fieldNamed(s.Types[arg.Type.base()].InputFields, "enabled"); ok {
			if _, provided := input["enabled"]; !provided {
				input["enabled"] = false
			}
		}
	}
	if e = s.validate(arg.Type, input, "input"); e != nil {
		return e
	}
	if o.Wait {
		if input["enabled"] != true {
			return fmt.Errorf("--wait on apply requires enabled:true")
		}
		if e = s.canWait(a); e != nil {
			return e
		}
	}
	vars := map[string]any{"input": input}
	if existing != nil {
		vars["name"] = name
	}
	if validateOnly || o.DryRun {
		changes := deviceDiff(base, input, "")
		return printJSON(map[string]any{"success": true, "validation": "schema only; broker semantic checks occur on apply", "operation": a.Root + "." + op, "device": name, "dryRun": o.DryRun, "changes": changes, "input": redactDevice(input)})
	}
	res, e := s.invoke(ctx, c, a, op, vars, false)
	if e != nil {
		return e
	}
	if o.Wait {
		return waitDevice(ctx, c, s, a, name, true, o.Timeout)
	}
	return printJSON(map[string]any{"success": true, "device": name, "operation": a.Root + "." + op, "result": res})
}
func deviceDiff(before, after map[string]any, prefix string) []any {
	out := []any{}
	keys := []string{}
	for k := range after {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		a, aok := before[k].(map[string]any)
		b, bok := after[k].(map[string]any)
		if aok && bok {
			out = append(out, deviceDiff(a, b, path)...)
			continue
		}
		old, _ := json.Marshal(before[k])
		next, _ := json.Marshal(after[k])
		if string(old) != string(next) {
			safe := redactDevice(map[string]any{k: after[k]}).(map[string]any)
			prior := redactDevice(map[string]any{k: before[k]}).(map[string]any)
			out = append(out, map[string]any{"path": path, "before": prior[k], "after": safe[k]})
		}
	}
	return out
}
func deviceAddress(ctx context.Context, c *Client, s *deviceSchema, a deviceAdapter, d map[string]any, p []string) error {
	action, name := p[0], fmt.Sprint(d["name"])
	if action == "list" {
		if len(p) != 2 {
			return fmt.Errorf("usage: device address list <name>")
		}
		cfg, _ := d["config"].(map[string]any)
		addresses := cfg["addresses"]
		if addresses == nil {
			addresses = []any{}
		}
		return printJSON(redactDevice(addresses))
	}
	op := ""
	switch action {
	case "add":
		op = "addAddress"
		if len(p) != 3 {
			return fmt.Errorf("usage: device address add <name> <file|->")
		}
	case "update":
		op = "updateAddress"
		if len(p) != 4 {
			return fmt.Errorf("usage: device address update <name> <key> <file|->")
		}
	case "delete":
		op = "deleteAddress"
		if len(p) != 3 {
			return fmt.Errorf("usage: device address delete <name> <key>")
		}
	default:
		return fmt.Errorf("unknown address action %q", action)
	}
	f, e := s.operation(a.Root, op)
	if e != nil {
		return e
	}
	vars := map[string]any{}
	keySet := false
	for _, arg := range f.Args {
		switch arg.Name {
		case "deviceName", "serverName":
			vars[arg.Name] = name
		case "input":
			path := p[len(p)-1]
			obj, e := readDeviceJSON(path)
			if e != nil {
				return e
			}
			vars[arg.Name] = obj
		default:
			if action != "add" && arg.Type.base() == "String" && !keySet {
				vars[arg.Name] = p[2]
				keySet = true
			} else {
				return fmt.Errorf("this address API requires native arguments; use device schema %s %s and device call", a.Type, op)
			}
		}
	}
	res, e := s.invoke(ctx, c, a, op, vars, false)
	if e != nil {
		return e
	}
	return printJSON(map[string]any{"success": true, "device": name, "result": res})
}
func (s *deviceSchema) canWait(a deviceAdapter) error {
	if a.Root == "opcUaDevice" {
		if _, ok := fieldNamed(s.Types[s.Query].Fields, "opcuaServers"); ok {
			return nil
		}
	}
	f, ok := fieldNamed(s.Types[s.Query].Fields, a.Query)
	if ok && s.hasConnectivity(f.Type, 0) {
		return nil
	}

	return fmt.Errorf("%s API does not expose runtime connectivity; use device status and verify topic values instead of --wait", a.Type)
}
func (s *deviceSchema) deviceStatus(ctx context.Context, c *Client, a deviceAdapter, name string) (map[string]any, error) {
	if a.Root == "opcUaDevice" {
		if _, ok := fieldNamed(s.Types[s.Query].Fields, "opcuaServers"); ok {
			data, e := deviceQuery(ctx, c, `query { opcuaServers { id name enabled connected } }`, nil)
			if e != nil {
				return nil, e
			}
			if list, ok := data["opcuaServers"].([]any); ok {
				for _, v := range list {
					if d, ok := v.(map[string]any); ok && d["name"] == name {
						return map[string]any{"device": d, "runtimeKnown": true}, nil
					}
				}
			}
			return map[string]any{"device": name, "runtimeKnown": false, "message": "Device is not present in runtime browser results"}, nil
		}
	}
	f, ok := fieldNamed(s.Types[s.Query].Fields, a.Query)
	if !ok {
		return nil, fmt.Errorf("status API unavailable for %s", a.Type)
	}
	selection := s.statusSelection(f.Type, 0)
	if selection == "" {
		return nil, fmt.Errorf("no status fields available for %s", a.Type)
	}
	vars := map[string]any{}
	def, use := "", ""
	if arg, ok := fieldNamed(f.Args, "name"); ok {
		def = "($name: " + arg.Type.String() + ")"
		use = "(name: $name)"
		vars["name"] = name
	}
	data, e := deviceQuery(ctx, c, "query DeviceStatus"+def+" { "+a.Query+use+" { "+selection+" } }", vars)
	if e != nil {
		return nil, e
	}
	items, ok := data[a.Query].([]any)
	if !ok {
		items = []any{data[a.Query]}
	}
	for _, item := range items {
		if d, ok := item.(map[string]any); ok && d["name"] == name {
			_, known := runtimeConnectivity(d)
			return map[string]any{"device": d, "runtimeKnown": known}, nil
		}
	}
	return nil, fmt.Errorf("device %q not returned by status API", name)
}
func waitDevice(ctx context.Context, c *Client, s *deviceSchema, a deviceAdapter, name string, connected bool, timeout time.Duration) error {
	if e := s.canWait(a); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		res, e := s.deviceStatus(ctx, c, a, name)
		if e != nil {
			return fmt.Errorf("configuration may already be applied; status verification failed: %w", e)
		}
		value, known := runtimeConnectivity(res["device"])
		if known && value == connected {
			return printJSON(res)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %s connected=%t; configuration may already be applied", name, connected)
		case <-timer.C:
		}
	}
}

// Runtime readiness is based only on explicit connectivity Booleans, never enabled or traffic.
func (s *deviceSchema) hasConnectivity(ref gqlRef, depth int) bool {
	if depth > 4 {
		return false
	}
	for _, f := range s.Types[ref.base()].Fields {
		if (f.Name == "connected" || f.Name == "isConnected") && f.Type.base() == "Boolean" {
			return true
		}
		switch f.Name {
		case "metrics", "status", "connectionStatus":
			if s.hasConnectivity(f.Type, depth+1) {
				return true
			}
		}
	}
	return false
}
func runtimeConnectivity(v any) (bool, bool) {
	switch x := v.(type) {
	case map[string]any:
		for _, key := range []string{"connected", "isConnected"} {
			if value, ok := x[key].(bool); ok {
				return value, true
			}
		}
		for _, key := range []string{"metrics", "status", "connectionStatus"} {
			if value, known := runtimeConnectivity(x[key]); known {
				return value, true
			}
		}
	case []any:
		if len(x) == 0 {
			return false, false
		}
		all := true
		for _, item := range x {
			value, known := runtimeConnectivity(item)
			if !known {
				return false, false
			}
			all = all && value
		}
		return all, true
	}
	return false, false
}
