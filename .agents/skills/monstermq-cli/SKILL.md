---
name: monstermq-cli
description: Comprehensive operational guide and CLI reference for mmq (MonsterMQ CLI tool). Use this skill when interacting with, testing, or building features for Full MonsterMQ Broker or MonsterMQ Edge Broker instances.
---

# MonsterMQ CLI (`mmq`) Skill Guide

This skill provides operational instructions and command references for using `mmq` to interact with MonsterMQ Broker and MonsterMQ Edge Broker GraphQL endpoints.

---

## Quick Reference & Binary Location

- **Binary Location**: `cli/bin/mmq` (built via `cd cli && ./build.sh` or `cd cli && go build -o bin/mmq .`)
- **Default Endpoint**: `http://localhost:4000/graphql`
- **Configuration Sources**:
  - CLI flags: `--url`, `--host`, `--port`, `--https`, `--user`, `--pass`, `--token`, `-i`
  - Environment variables: `MQ_URL`, `MQ_HOST`, `MQ_PORT`, `MQ_HTTPS`, `MQ_USER`, `MQ_PASS`, `MQ_TOKEN` (or `GRAPHQL_*` equivalents)
  - `.env` files

---

## Interactive Shell / REPL Mode

`mmq` can run as an interactive REPL shell, maintaining connection and authentication state so commands can be executed repeatedly without restarting the executable:

```bash
# Launch interactive REPL directly on default broker (localhost:4000)
mmq

# Connect to a specific port or remote host:
mmq --port 4001
mmq --host 192.168.1.50 --port 4001
mmq --host secure-broker --https

# Or explicitly with shell subcommand:
mmq shell
mmq --url http://192.168.1.50:4000/graphql shell
```

### REPL Built-in Commands
When inside the interactive prompt (`mmq [host]>`):
- **`connect <url|host:port|port>`**: Switch or reconnect to a different broker endpoint (e.g. `connect 4001`, `connect 192.168.1.50:4001`, or full URL).
- **`auth <user> <pass>`** / **`login <user> <pass>`**: Authenticate in session and acquire JWT token.
- **`token <jwt>`**: Set or update JWT token.
- **`status`**: Inspect broker connection health, authentication state (`username`, `isAdmin`), and enabled features.
- **`json [on|off]`**: Toggle or inspect JSON output mode on the fly.
- **`clear`**: Clear the terminal screen.
- **`help [command]`**: Display REPL interactive help menu.
- **`exit`** / **`quit`** (or `Ctrl+D`): Exit the interactive shell.

---

## Operational Workflows

### 1. Endpoint Discovery & Feature Inspection
Before running complex queries against a target MonsterMQ node, inspect its enabled features to determine whether it is a central **Full Broker** or an **Edge Broker**:

```bash
mmq --port 4001 features
```

- **Full Broker**: Supports multi-group archives (`archiveGroups`), historical message logs (`archivedMessages`), and TSDB aggregations (`aggregatedMessages`).
- **Edge Broker**: Focuses on real-time topic value reading/writing, edge device configuration sync, and MQTT client toggling.

---

## Command Reference

### 1:1 GraphQL Commands
`mmq` maps 1:1 to MonsterMQ GraphQL functions (executable either as single CLI commands or inside the interactive shell):
- **`searchTopics [pattern]`**: Search active topics (globs `*`, SQL `%`, MQTT `#`)
- **`currentValue <topic>`**: Get current or retained value for a single topic
- **`currentValues <filter>`**: Get current values matching a topic filter
- **`retainedMessages [filter]`**: List retained messages matching a topic filter
- **`browseTopics [path]`**: Browse topic hierarchy level-by-level
- **`publish <topic> [payload]`**: Publish message payload (`--retain`, `--qos 0|1|2`, defaults to empty `""` if omitted)
- **`subscribe <topics...>`**: Subscribe to real-time topic updates via WebSocket (`--ws-url`, `--format`, `--raw`)
- **`monitor <topics...>`**: Interactive live dashboard table to view incoming topic values in-place with scrolling
- **`archivedMessages <topic> [archiveGroup]`**: Query historical time-series messages (defaults to last 60s window)
- **`aggregatedMessages <topics...>`**: Query server-side time-series aggregations (`AVG`, `MIN`, `MAX`)
- **`archiveGroups`**: List all deployed archive storage groups
- **`archiveStats <group>`**: Get stats for an archive group
- **`systemLogs`**: View broker system log entries (`--last-minutes N`)
- **`sessions`**: List active MQTT client sessions
- **`session <clientId>`**: Inspect specific client session details
- **`currentUser`**: Get authenticated user and admin status
- **`databaseConnections`**: List configured database connections
- **`hmis` / `hmi list`**: List deployed HMI web dashboards
- **`hmi create <name> [options]`**: Create a new HMI dashboard definition
- **`hmi remove <name...>`**: Delete and remove one or more deployed HMI dashboards
- **`exportHmiZip <name> [out] [--unzip]`** (or `hmi export`): Export deployed HMI package to binary zip file or extract to folder
- **`importHmiZip <file.zip|dir> [name] [--main]`** (or `hmi import`): Upload & deploy HMI dashboard from a zip package or local directory
- **`brokerConfig`**: List enabled broker features & capabilities

### Device Configuration for AI Workflows

Use dedicated device APIs for configuration; import/export is a separate backup workflow.

1. `mmq --json device types`: discover target API operations and enabled features. API presence does not mean the feature is enabled on the assigned node. Edge has fewer protocols than Full Broker.
2. `mmq --json device schema <type> [operation]`: obtain native arguments, required fields, defaults, descriptions and nested input/enum types. Default operation is create/add.
3. `mmq device template <type>`: generate `{ "type": "MQTT-Client", "input": { ... } }`; fill connection fields and choose node assignment. `*` uses broker assignment semantics and can target all nodes.
4. `mmq --json device validate <file|->` and `device apply <file|-> --dry-run`: inspect schema validation, merged input and redacted field changes without mutations.
5. `mmq --json device apply <file|->`: create/update through the native API. New devices default to disabled. Existing devices retain omitted fields, credentials and enabled state. Objects merge recursively; arrays replace. Type changes are rejected. Read/merge/write is not atomic; avoid concurrent writers.
6. Configure addresses, enable, and verify runtime state/topic values.

Supported adapters (when exposed by the broker): MQTT, OPC UA, Kafka, WinCC OA, WinCC Unified, PLC4X, NATS, Redis, Neo4j, Telegram, i3X clients; OPC UA/Kafka servers; JDBC/InfluxDB/TimeBase loggers; Sparkplug B decoders. Type aliases ignore case, hyphens and underscores. HMI commands remain separate. The CLI cannot add a missing protocol implementation or change broker YAML/listener settings through device commands.

Commands:
- `device list [type] [--type <type>]`: list/filter device configurations.
- `device get <name>`: read configuration with password/secret/token/private-key fields redacted.
- `device enable|disable <name> [--wait] [--timeout 30s]`: use native toggle, never import.
- `device delete <name>`: invoke native delete.
- `device status <name> [--wait] [--timeout 30s]`: report operational state with `runtimeKnown`. Enabled does not imply connected.
- `device browse <name> [nodeId]`: OPC UA browsing, default Objects `i=85`.
- `device address list <name>`: read configured mappings.
- `device address add <name> <file|->`: native address input object.
- `device address update <name> <key> <file|->`: native update if supported.
- `device address delete <name> <key>`: native deletion. Keys depend on protocol: MQTT remoteTopic, OPC UA address, WinCC OA query, Unified topic, i3X elementId, etc.
- `device call <type> <operation> <args-file|-> [--dry-run]`: invoke additional native operations such as reassign, start/stop, certificate management or decoder rules. Discover exact arguments with `device schema <type> <operation>`. Call uses native defaults and does NOT merge configuration or default devices to disabled.
- `device download [name] [output.json]`: raw backup export, including credentials; newly created files are owner-only.
- `device upload <backup.json|->`: backup import, accepting a single object or array. Both brokers force imported devices to disabled. Partial failures produce nonzero exits.

MQTT and OPC UA connection inputs do not accept address arrays; addresses are managed separately and preserved on update. OPC UA has add/delete but currently no updateAddress API. Do not silently emulate updates by deleting mappings; use explicit operations when requested.

Validation checks schema shape, scalar types, lists, enums and required/unknown fields; broker semantic/range validation occurs on mutation. It does not test connectivity. `apply --wait` requires enabled:true. Waiting requires an exposed runtime connectivity Boolean; unsupported waits are rejected before mutation. Current MQTT device APIs do not expose this Boolean, so inspect status and topic values instead. `--timeout` requires `--wait`, accepts a positive duration and defaults to 30s. `--dry-run` cannot be combined with `--wait`. A timeout does not undo a mutation.

All JSON file inputs accept stdin as `-`. Apply files use `{type,input}` and are different from exported backup files. Do not apply redacted outputs: `[REDACTED]` placeholders are rejected; omit credential fields to preserve them. Explicit null follows broker clearing/preservation semantics.

New device commands require GraphQL introspection; name-based lookup/apply additionally require DeviceImportExport. New commands emit JSON. Use global `--json` BEFORE `device` for structured failure output `{success:false,error:...}` and check process exit codes. Do not assume a successful mutation proves the connector is connected.

Examples from the repository (edit endpoints, addresses and assignments first):
```bash
mmq --json device apply cli/examples/devices/mqtt.json --dry-run
mmq --json device apply cli/examples/devices/mqtt.json
mmq --json device address add mqtt-example cli/examples/devices/mqtt-address.json
mmq --json device enable mqtt-example
mmq --json device status mqtt-example
mmq --json device apply cli/examples/devices/opcua.json
mmq --json device enable opcua-example
mmq --json device browse opcua-example
mmq --json device address add opcua-example cli/examples/devices/opcua-address.json
```

See `cli/README.md` for the complete reference and configuration semantics.

### JSON Output & Automation
For automated processing or shell pipelines, put `--json` before the command:
```bash
mmq --json searchTopics "sensors/#"
```

---

## Maintenance & Skill Synchronization

> **MANDATORY**: Whenever a subcommand, flag, GraphQL query, or feature capability is added, modified, or deprecated in `mmq`, this `SKILL.md` file **MUST** be updated to reflect the change.

## GraphQL contract freshness

The generator lives in `tools/scripts/graphql-contract/`, launched using
`tools/scripts/graphql-contract.sh`. It reads the Full runtime schema list and
Edge gqlgen inputs, validates literal CLI operations and the adapter registry.
It writes no schema copies into the CLI repository.

```bash
# From tools:
./scripts/graphql-contract.sh --edge-root ../edge --cli-root . --output-dir /tmp/mmq-contract
(cd cli && MMQ_CONTRACT_DIR=/tmp/mmq-contract go test ./...)
```

`--output-dir` overrides the default `main/doc/graphql/` output, which contains
separate `main/` and `edge/` directories. `--check` verifies generated outputs
without writing. `--cli-root` validates CLI compatibility only.

`MMQ_CONTRACT_DIR` is test-only: a directory containing `main/` and `edge/`, each
with `schema.graphql` and `introspection.json`. When omitted, CLI device tests
create temporary contracts from sibling main/edge source checkouts and remove
temporary files after loading. Missing sources/contracts fail tests explicitly.
CI generates contracts in its temporary directory. Do not recreate stored CLI
schema copies.

Runtime `device schema` uses live introspection. GraphQL `#` comments are not
introspection descriptions; use triple-quoted descriptions for AI-visible help.
New groups/semantics still require adapter tests. Session removal uses
`session.removeSessions.results`. Use `--check --live-url <url>` for source/live
shape comparison, with `GRAPHQL_CONTRACT_TOKEN` if needed. It does not verify
mutation behavior, permissions or connector readiness.
