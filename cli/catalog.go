package main

// Machine-readable command catalog: `mmq commands [--json] [name]`.
// Keep in sync with ExecuteCommand (TestCatalogCoversAllCommands enforces this).

import (
	"fmt"
	"sort"
	"strings"
)

type catalogOption struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type catalogCommand struct {
	Name        string          `json:"name"`
	Aliases     []string        `json:"aliases,omitempty"`
	Summary     string          `json:"summary"`
	Usage       string          `json:"usage"`
	Group       string          `json:"group"`
	Brokers     string          `json:"brokers"`  // both | full | edge
	Mutating    bool            `json:"mutating"` // changes broker state
	Input       string          `json:"input"`    // flags | json | graphql
	Options     []catalogOption `json:"options,omitempty"`
	Examples    []string        `json:"examples,omitempty"`
	Subcommands []string        `json:"subcommands,omitempty"`
}

func opts(kv ...string) []catalogOption {
	out := []catalogOption{}
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, catalogOption{kv[i], kv[i+1]})
	}
	return out
}

var apiOpts = opts(
	"--args <json|@file|->", "arguments as JSON object (or array when the operation has a single list argument)",
	"key=value", "single argument; flat fields allowed when the operation takes one input object",
	"--select", "override result selection set",
	"--depth N", "selection depth (default 3)",
	"--dry-run", "print generated GraphQL, do not send",
	"--reveal", "do not redact secrets in output")

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func nounCatalog() []catalogCommand {
	out := []catalogCommand{}
	for _, n := range nounSpecs {
		verbs := []string{}
		for v := range n.Queries {
			verbs = append(verbs, v)
		}
		sort.Strings(verbs)
		for v := range n.Mutations {
			verbs = append(verbs, v)
		}
		sort.Strings(verbs)
		first := "--help"
		if len(verbs) > 0 {
			first = verbs[0]
		}
		if n.Mutation != "" {
			verbs = append(verbs, "<mutation>")
		}
		out = append(out, catalogCommand{
			Name: n.Names[0], Aliases: n.Names[1:], Summary: n.Help, Group: "Administration",
			Usage:   "mmq " + n.Names[0] + " <verb> [--args json|@file|-] [key=value ...]",
			Brokers: firstNonEmpty(n.Brokers, "full"), Mutating: n.Mutation != "" || len(n.Mutations) > 0, Input: "json", Options: apiOpts, Subcommands: verbs,
			Examples: []string{"mmq schema " + firstNonEmpty(n.Mutation, n.Names[0]), "mmq --json " + n.Names[0] + " " + first},
		})
	}
	return out
}

var baseCatalog = []catalogCommand{
	// Topics & values
	{Name: "searchTopics", Group: "Topics", Summary: "Search active topics (globs *, SQL %, MQTT #)", Usage: "mmq searchTopics [pattern] [--limit N] [--archive-group G]", Brokers: "both", Input: "flags", Options: opts("--limit, -l", "max results (100)", "--archive-group, -g", "archive group (Default)")},
	{Name: "currentValue", Group: "Topics", Summary: "Current or retained value of one topic", Usage: "mmq currentValue <topic> [archiveGroup]", Brokers: "both", Input: "flags", Options: opts("--archive-group, -g", "archive group")},
	{Name: "currentValues", Group: "Topics", Summary: "Current values matching a topic filter", Usage: "mmq currentValues <filter> [--limit N]", Brokers: "both", Input: "flags", Options: opts("--limit, -l", "max results (100)", "--archive-group, -g", "archive group")},
	{Name: "retainedMessages", Group: "Topics", Summary: "List retained messages", Usage: "mmq retainedMessages [filter] [--limit N]", Brokers: "both", Input: "flags", Options: opts("--limit, -l", "max results (100)")},
	{Name: "browseTopics", Group: "Topics", Summary: "Browse topic hierarchy level by level", Usage: "mmq browseTopics [path]", Brokers: "both", Input: "flags", Options: opts("--archive-group, -g", "archive group")},
	{Name: "publish", Group: "Topics", Summary: "Publish a message (empty payload + --retain clears a retained topic)", Usage: "mmq publish <topic> [payload] [--retain] [--qos 0|1|2]", Brokers: "both", Mutating: true, Input: "flags", Options: opts("--retain, -r", "retain flag", "--qos", "0, 1 or 2")},
	{Name: "publishBatch", Group: "Topics", Summary: "Publish many messages in one call", Usage: `mmq publishBatch --args '[{"topic":"a/b","payload":"1","qos":0,"retain":false}]'`, Brokers: "full", Mutating: true, Input: "json", Options: apiOpts},
	{Name: "purgeQueuedMessages", Group: "Sessions", Summary: "Purge queued messages of a client or of all clients", Usage: "mmq purgeQueuedMessages [clientId=<id>]", Brokers: "full", Mutating: true, Input: "json", Options: apiOpts},
	// Real time
	{Name: "subscribe", Group: "Real-time", Summary: "Stream topic updates over GraphQL WebSocket (agent friendly)", Usage: "mmq subscribe <topics...> [--format JSON|TEXT|BINARY] [--raw]", Brokers: "both", Input: "flags", Options: opts("--ws-url, -w", "WebSocket URL", "--format, -f", "JSON, TEXT, BINARY", "--raw", "do not decode base64", "--monitor, -m", "switch to live table")},
	{Name: "monitor", Group: "Real-time", Summary: "Interactive full-screen live table (TTY only; use subscribe for agents)", Usage: "mmq monitor [topics...]", Brokers: "both", Input: "flags"},
	// Archives
	{Name: "archivedMessages", Group: "Archives", Summary: "Historical messages of a topic", Usage: "mmq archivedMessages <topic> [group] [--last-seconds N | --start ISO --end ISO] [--limit N]", Brokers: "full", Input: "flags", Options: opts("--start/--end", "ISO 8601 UTC", "--last-seconds, -s", "relative window", "--limit, -l", "max rows (100)", "--archive-group, -g", "group")},
	{Name: "aggregatedMessages", Group: "Archives", Summary: "Server-side time-series aggregation", Usage: "mmq aggregatedMessages <topics...> [--interval ONE_MINUTE|FIVE_MINUTES|FIFTEEN_MINUTES|ONE_HOUR|ONE_DAY] [--functions AVG,MIN,MAX,COUNT] [--fields f1,f2]", Brokers: "full", Input: "flags"},
	{Name: "archiveGroups", Group: "Archives", Summary: "List archive groups (CRUD: mmq archiveGroup <verb>)", Usage: "mmq archiveGroups", Brokers: "both", Input: "flags"},
	{Name: "archiveStats", Group: "Archives", Summary: "Min timestamps and daily counts of an archive group", Usage: "mmq archiveStats <group> [--last-seconds N]", Brokers: "full", Input: "flags"},
	// Broker
	{Name: "brokerConfig", Aliases: []string{"features"}, Group: "Broker", Summary: "Enabled features/capabilities (run first to tell Full from Edge)", Usage: "mmq features", Brokers: "both", Input: "flags"},
	{Name: "systemLogs", Group: "Broker", Summary: "Broker log entries", Usage: "mmq systemLogs [--last-minutes N] [--limit N]", Brokers: "both", Input: "flags"},
	{Name: "currentUser", Group: "Broker", Summary: "Authenticated user and admin flag", Usage: "mmq currentUser", Brokers: "both", Input: "flags"},
	{Name: "databaseConnections", Group: "Broker", Summary: "Configured database connections", Usage: "mmq databaseConnections [POSTGRES|MONGODB|SQLITE]", Brokers: "full", Input: "flags"},
	{Name: "sessions", Group: "Sessions", Summary: "List MQTT client sessions", Usage: "mmq sessions", Brokers: "both", Input: "flags"},
	{Name: "session", Group: "Sessions", Summary: "Inspect or remove client sessions", Usage: "mmq session <clientId> | session list | session remove <clientId...>", Brokers: "both", Mutating: true, Input: "flags", Subcommands: []string{"list", "inspect", "remove"}},
	// HMI
	{Name: "hmi", Aliases: []string{"hmis"}, Group: "HMI", Summary: "HMI dashboards: list/create/remove/export/import/sync", Usage: "mmq hmi <list|create|remove|export|import|sync> ...", Brokers: "both", Mutating: true, Input: "flags", Subcommands: []string{"list", "create", "remove", "export", "import", "sync"}},
	{Name: "exportHmiZip", Aliases: []string{"downloadHmiZip"}, Group: "HMI", Summary: "Export an HMI to zip/folder", Usage: "mmq exportHmiZip <name> [out] [--unzip]", Brokers: "both", Input: "flags"},
	{Name: "importHmiZip", Aliases: []string{"uploadHmiZip"}, Group: "HMI", Summary: "Deploy an HMI from zip/folder", Usage: "mmq importHmiZip <file.zip|dir> [name] [--main]", Brokers: "both", Mutating: true, Input: "flags"},
	{Name: "hmiSync", Aliases: []string{"sync"}, Group: "HMI", Summary: "Live-sync an HMI folder over MQTT", Usage: "mmq hmi sync <name> [dir] [--pull|--pull-only|--push-only|--dry-run]", Brokers: "both", Mutating: true, Input: "flags"},
	// Devices & scripts
	{Name: "device", Group: "Devices", Summary: "Schema-driven connector/logger/decoder configuration", Usage: "mmq --json device <types|schema|template|validate|apply|list|get|enable|disable|delete|status|browse|address|call|download|upload>", Brokers: "both", Mutating: true, Input: "json", Subcommands: []string{"types", "schema", "template", "validate", "apply", "list", "get", "enable", "disable", "delete", "status", "browse", "address", "call", "download", "upload"}, Examples: []string{"mmq --json device template MQTT-Client > dev.json", "mmq --json device apply dev.json --dry-run"}},
	{Name: "script", Aliases: []string{"scripts"}, Group: "Scripts", Summary: "Broker scripts: lifecycle, dry-run test, logs, docs, skill", Usage: "mmq script <list|get|create|update|delete|toggle|test|logs|docs|skill|languages> ...", Brokers: "both", Mutating: true, Input: "flags", Subcommands: []string{"list", "get", "create", "update", "delete", "toggle", "enable", "disable", "test", "logs", "docs", "skill", "languages"}},
	// Generic
	{Name: "api", Group: "Generic", Summary: "Call ANY query/mutation by path with schema validation", Usage: "mmq api <path> [--args json|@file|-] [key=value ...]", Brokers: "both", Mutating: true, Input: "json", Options: apiOpts, Subcommands: []string{"describe", "template", "list"}, Examples: []string{"mmq api describe user.createUser", "mmq api template user.createUser", "mmq api users"}},
	{Name: "gql", Aliases: []string{"graphql"}, Group: "Generic", Summary: "Send a raw GraphQL document", Usage: "mmq gql <query|@file|-> [--vars json|@file|-]", Brokers: "both", Mutating: true, Input: "graphql", Examples: []string{"mmq gql '{ brokers { nodeId } }'"}},
	{Name: "schema", Group: "Generic", Summary: "Introspect operations and types of the connected broker", Usage: "mmq schema [filter|Type|path] [--queries|--mutations]", Brokers: "both", Input: "flags"},
	{Name: "commands", Aliases: []string{"catalog"}, Group: "Generic", Summary: "This catalog (machine readable with --json)", Usage: "mmq commands [name] [--json]", Brokers: "both", Input: "flags"},
	{Name: "skill", Group: "Generic", Summary: "Generate or install the agent skill (SKILL.md) from this catalog", Usage: "mmq skill [--output file] [--install [dir]]", Brokers: "both", Input: "flags"},
	{Name: "shell", Aliases: []string{"repl", "interactive", "sh"}, Group: "Generic", Summary: "Interactive REPL", Usage: "mmq shell", Brokers: "both", Input: "flags"},
}

func fullCatalog() []catalogCommand {
	all := append([]catalogCommand{}, baseCatalog...)
	all = append(all, nounCatalog()...)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Group != all[j].Group {
			return all[i].Group < all[j].Group
		}
		return all[i].Name < all[j].Name
	})
	return all
}

type catalogDoc struct {
	Tool          string            `json:"tool"`
	GlobalOptions []catalogOption   `json:"globalOptions"`
	Environment   []string          `json:"environment"`
	ExitCodes     map[string]string `json:"exitCodes"`
	ErrorShape    string            `json:"errorShape"`
	Commands      []catalogCommand  `json:"commands"`
}

func buildCatalogDoc() catalogDoc {
	return catalogDoc{
		Tool: "mmq",
		GlobalOptions: opts(
			"--url", "GraphQL endpoint (default http://localhost:4000/graphql)",
			"--host/--port/--https", "endpoint parts",
			"--user/-u, --pass/-p, --token", "credentials / JWT",
			"--mqtt-host/--mqtt-port/--mqtt-user/--mqtt-pass", "MQTT settings (hmi sync)",
			"--env <file>", ".env file (default .env)",
			"--json", "machine-readable output and errors; accepted anywhere on the command line"),
		Environment: []string{"MQ_URL", "MQ_HOST", "MQ_PORT", "MQ_HTTPS", "MQ_USER", "MQ_PASS", "MQ_TOKEN", "MQ_MQTT_HOST", "MQ_MQTT_PORT", "MQ_MQTT_USER", "MQ_MQTT_PASS"},
		ExitCodes:   map[string]string{"0": "success", "1": "command failed (GraphQL error, success:false, I/O, validation)", "2": "invalid global options"},
		ErrorShape:  `With --json a failure prints {"success":false,"error":"<message>"} to stdout (and "Error: <message>" to stderr).`,
		Commands:    fullCatalog(),
	}
}

func runCommands(c *Client, args []string) error {
	var name string
	for _, a := range args {
		switch a {
		case "--json":
			c.cfg.JSONMode = true
		case "-h", "--help":
			fmt.Println("Usage: mmq commands [name] [--json]\nList every command with usage, options, broker support and input style.")
			return nil
		default:
			name = a
		}
	}
	doc := buildCatalogDoc()
	if name != "" {
		for _, cmd := range doc.Commands {
			hit := strings.EqualFold(cmd.Name, name)
			for _, al := range cmd.Aliases {
				hit = hit || strings.EqualFold(al, name)
			}
			if !hit {
				continue
			}
			if c.cfg.JSONMode {
				return printJSON(cmd)
			}
			fmt.Printf("%s - %s\nUsage: %s\nBrokers: %s  Mutating: %v  Input: %s\n", cmd.Name, cmd.Summary, cmd.Usage, cmd.Brokers, cmd.Mutating, cmd.Input)
			for _, o := range cmd.Options {
				fmt.Printf("  %-28s %s\n", o.Name, o.Description)
			}
			return nil
		}
		return fmt.Errorf("unknown command %q", name)
	}
	if c.cfg.JSONMode {
		return printJSON(doc)
	}
	group := ""
	for _, cmd := range doc.Commands {
		if cmd.Group != group {
			group = cmd.Group
			fmt.Printf("\n%s\n", group)
		}
		fmt.Printf("  %-22s %s [%s]\n", cmd.Name, cmd.Summary, cmd.Brokers)
	}
	return nil
}
