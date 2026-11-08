---
name: monstermq-cli
description: Comprehensive operational guide and CLI reference for mmq (MonsterMQ CLI tool). Use this skill when interacting with, testing, or building features for Full MonsterMQ Broker or MonsterMQ Edge Broker instances.
---

# MonsterMQ CLI (`mmq`) Skill Guide

This skill provides operational instructions and command references for using `mmq` to interact with MonsterMQ Broker and MonsterMQ Edge Broker GraphQL endpoints.

---

## Quick Reference & Binary Location

- **Binary Location**: `cli/bin/mmq` / `cli/bin/mmq.exe` (built via `python make.py build --cli` from the tools repo root, or `cd cli && go build -o bin/mmq .`)
- **Default Endpoint**: `http://localhost:4000/graphql`
- **Configuration Sources**:
  - CLI flags (must come **before** the command): `--url`, `--host`, `--port`, `--https`, `--user`/`--username`/`-u`, `--pass`/`--password`/`-p`, `--token`, `--mqtt-host`, `--mqtt-port`, `--mqtt-user`, `--mqtt-pass` (MQTT credentials default to the CLI auth user/pass), `--env`/`--env-file <file>` (default `.env`), `--json`, `-i`/`--interactive`, `-h`/`--help`
  - Environment variables: `MQ_URL`, `MQ_HOST`, `MQ_PORT`, `MQ_HTTPS`, `MQ_USER`, `MQ_PASS`, `MQ_TOKEN` (or `GRAPHQL_*` equivalents); `MQ_MQTT_HOST`/`MQTT_HOST`, `MQ_MQTT_PORT`/`MQTT_PORT`, `MQ_MQTT_USER`/`MQTT_USER`, `MQ_MQTT_PASS`/`MQTT_PASS`
  - `.env` files (values only apply when the variable is not already set in the environment)
  - `--json` can be placed anywhere; machine-readable catalog: `mmq commands --json`; exit codes `0` ok / `1` failed / `2` bad options.
  - Every command supports `-h`/`--help` for its own usage text.

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

# Or explicitly with shell subcommand (aliases: repl, interactive, sh) or -i:
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
- **`clear`** / **`cls`**: Clear the terminal screen.
- **`help [command]`** / **`?`**: Display REPL interactive help menu.
- **`exit`** / **`quit`** / **`q`** (or `Ctrl+D`): Exit the interactive shell.
- Any other input is executed as a regular `mmq` command (e.g. `searchTopics "sensors/#"`).

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
**Topics & values**
- **`searchTopics [pattern]`**: Search active topics (globs `*`, SQL `%`, MQTT `#`; default `#`). Options: `--limit N`/`-l` (default 100), `--archive-group`/`-g` (default `Default`)
- **`currentValue <topic> [archiveGroup]`**: Get current or retained value for a single topic (`--archive-group`/`--group`/`-g`)
- **`currentValues <filter>`**: Get current values matching a topic filter (`--limit`/`-l`, default 100; `--archive-group`/`-g`)
- **`retainedMessages [filter]`**: List retained messages matching a topic filter (default `#`; `--limit`/`-l`, default 100)
- **`browseTopics [path]`**: Browse topic hierarchy level-by-level (default `+`; `--archive-group`/`-g`)
- **`publish <topic> [payload]`**: Publish message payload (`--retain`/`-r`, `--qos 0|1|2`; payload defaults to empty `""`, useful for clearing retained topics)

**Real-time**
- **`subscribe <topics...>`**: Stream real-time topic updates via GraphQL WebSocket. Options: `--ws-url`/`-w <url>`, `--format`/`-f JSON|TEXT|BINARY` (default JSON), `--raw` (don't decode base64 payloads), `--monitor`/`-m` (switch to the live table below)
- **`monitor [topics...]`**: Interactive full-screen live table of topic values (default filter `#`; same `--ws-url`/`--format`/`--raw` options). Keys: `q`/`Esc`/Ctrl+C quit, `p` pause, `c` clear, `s` cycle sort (name → time → count), arrows or `k`/`j` and PgUp/PgDn scroll, `g`/`G` or Home/End top/bottom. Falls back to streaming output when stdin is not a TTY — for agents/scripts prefer `subscribe`.

**Archives (Full Broker)**
- **`archivedMessages <topic> [archiveGroup]`**: Query historical messages (default last 60s, `Default` group). Options: `--start`/`--end ISO_TIME` (ISO 8601 UTC), `--last-seconds`/`-s N`, `--limit`/`-l N` (default 100), `--archive-group`/`--group`/`-g`
- **`aggregatedMessages <topics...>`**: Server-side time-series aggregations. Options: `--interval ONE_MINUTE|FIVE_MINUTES|FIFTEEN_MINUTES|ONE_HOUR|ONE_DAY` (default ONE_MINUTE), `--functions AVG,MIN,MAX,COUNT` (default AVG), `--fields f1,f2` (JSON fields), `--start`/`--end`, `--last-seconds`/`-s`, `--archive-group`/`-g`
- **`archiveGroups`**: List all deployed archive storage groups
- **`archiveStats <group>`**: Min timestamps and daily message counts for an archive group (`--start`, `--end`, `--last-seconds`/`-s`)

**Broker, sessions & users**
- **`brokerConfig`** / **`features`**: List enabled broker features & capabilities
- **`systemLogs`**: View broker system log entries (`--last-minutes N`, default 60; `--limit N`, default 50)
- **`sessions`** / **`session list`**: List active MQTT client sessions
- **`session <clientId>`** / **`session inspect <clientId>`**: Inspect specific client session details and subscriptions
- **`session remove <clientId...>`**: Remove / disconnect one or more client sessions
- **`currentUser`**: Get authenticated user and admin status
- **`databaseConnections [type]`**: List configured database connections, optionally filtered by type (`POSTGRES`, `MONGODB`, `SQLITE`)

**HMI dashboards** (see also the `monstermq-hmi-builder` skill)
- **`hmis` / `hmi list`** (or `hmi ls`): List deployed HMI web dashboards
- **`hmi create <name> [options]`** (or `new`, `add`): Create a new HMI dashboard definition. Options: `--path <urlPath>` (default `/<name>`), `--title`, `--description`, `--entry-point <file>` (default `index.html`), `--main`/`-m` (primary dashboard), `--node <nodeId>`, `--disabled`
- **`hmi remove <name...>`** (or `delete`, `rm`): Delete one or more deployed HMI dashboards
- **`exportHmiZip <name> [out] [--unzip|-u]`** (or `hmi export`, `hmi download`, `downloadHmiZip`): Export deployed HMI package to a zip file (default `<name>.zip`) or extract to a folder
- **`importHmiZip <file.zip|dir> [name] [--main|-m]`** (or `hmi import`, `hmi upload`, `uploadHmiZip`): Upload & deploy an HMI dashboard from a zip package or local directory (`.git` and metadata are excluded when uploading folders). Name defaults to the zip/folder name; `importHmiZip <name> <file.zip|dir>` order is also accepted.
- **`hmi sync <name> [localDir]`** (or `hmi watch`, `sync`, `hmiSync`): Live file synchronization between a local directory (default `./<name>`) and the broker's HMI storage over MQTT, with cross-platform recursive watching. Options: `--pull` (pull remote files before watching), `--pull-only` (one-shot download), `--push-only` (one-shot upload), `--dry-run` (log only, no MQTT packets), `--debounce <ms>` (default 100), `--ignore <patterns>` (default `.git,node_modules,*.tmp`), `--base-topic <topic>` (default `monstermq/hmi/sync`), `--mqtt-host`, `--mqtt-port` (default 1883, 8883 for TLS), `--mqtt-user`, `--mqtt-pass`

**Scripts** (details in the section below)
- **`scripts` / `script list [filter]`** (or `ls`): List configured broker scripts with execution metrics, triggers, and state
- **`script get <name>`** (or `inspect`, `show`): Inspect full script configuration, bindings, source code, and circular execution logs
- **`script create <name> [options]`** (or `new`, `add`): Create and deploy a new script
- **`script update <name> [options]`** (or `edit`): Update existing script code, trigger parameters, or options
- **`script delete <name...>`** (or `rm`, `remove`): Delete one or more scripts
- **`script toggle <name> <on|off|true|false>`**, **`script enable|start <name>`**, **`script disable|stop <name>`**: Toggle script execution state
- **`script test <name|--code <code>> [options]`** (or `run`, `dry-run`): Dry-run a script in a sandbox with mock `--topic`, `--payload`, and `--args`
- **`script logs <name>`** (or `log`): Inspect recent circular execution log entries and errors
- **`script docs [language]`** (or `doc`): Fetch the authoritative Markdown API reference from the connected broker
- **`script skill [language] [--output|-o file] [--install [dir]]`** (or `skills`): Retrieve the broker's AI Skill; `--install` writes `SKILL.md` to `[dir]` or `~/.gemini/antigravity/skills/monstermq-scripts/` (fallback `./.agents/skills/monstermq-scripts/` if no home dir)
- **`script languages`** (or `langs`): Discover supported scripting languages (Starlark on Edge; Python 3 & JavaScript on Main)

**Devices**
- **`device <command>`**: Device/connector configuration (see next section)

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
- `device types`: discover supported device types, API operations and enabled broker features.
- `device schema <type> [operation]`: describe mutation arguments and nested input/enum types.
- `device template <type>`: generate an editable `{type,input}` JSON document.
- `device validate <file|->`: validate an apply document against the connected broker.
- `device apply <file|-> [--dry-run] [--wait] [--timeout 30s]`: merge supplied fields and create/update a device.
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

### Broker Script Management & AI Skills

`mmq script` provides complete lifecycle management for standalone broker scripts (Starlark on Edge Broker; Python 3 & JavaScript on MonsterMQ Core), dry-run testing in an interactive sandbox, execution log inspection, and dynamic AI Skill export.

#### 1. Discovery & Inspection
```bash
# List all configured scripts on the connected broker (or mmq scripts)
mmq script list
mmq script list "temp"

# Inspect detailed script configuration, trigger parameters, code, and recent execution logs
mmq script get TemperatureMonitor

# List supported languages on the target broker
mmq script languages
```

#### 2. Authoring, Deploying & Modifying
`create` / `update` options:
- `--lang <lang>`: `starlark`, `python`, `javascript` (check `script languages`)
- `--trigger TOPIC|TIMER|BOTH|CALLABLE` (default `TOPIC`)
- `--topic <filter>`: topic filter (repeatable or comma-separated)
- `--interval <ms>`: timer interval for `TIMER`/`BOTH`
- `--on-change`: trigger only when the payload changes (skip duplicate consecutive payloads)
- `--mode SINGLETON|MULTI_INSTANCE` (default `SINGLETON`)
- `--timeout <ms>`: execution timeout (default 200)
- `--code <code>` or `--file <path>`: script source
- `--desc <description>`
- `create` only: `--node <nodeId>` (default local or `*`), `--namespace <ns>` (default `script`), `--disabled`
- `update` only: `--no-on-change` (clear the on-change flag); use `script toggle` to enable/disable

Scripts can be deployed directly from an inline code string or from a local source file:
```bash
# Deploy a new script from inline code
mmq script create HighTempAlert \
  --lang starlark \
  --trigger TOPIC \
  --topic "sensors/+/temperature" \
  --desc "Publishes alerts when temperature exceeds threshold" \
  --code '
if msg != None and type(msg["payload"]) == "dict":
    temp = msg["payload"].get("temp", 0)
    if temp > 75.0:
        mqtt.publish("alarms/temp", json.encode({"alarm": True, "temp": temp}), qos=1)
        log.warn("High temp: " + str(temp))
'

# Deploy a script from a local file with a periodic timer trigger
mmq script create ChillerWatchdog \
  --lang starlark \
  --trigger TIMER \
  --interval 5000 \
  --file ./chiller.star

# Update existing script code or configuration
mmq script update HighTempAlert --file ./updated.star

# Toggle script execution state
mmq script toggle HighTempAlert on
mmq script toggle HighTempAlert off
# or using aliases: mmq script start <name>, mmq script stop <name>

# Delete scripts
mmq script delete HighTempAlert ChillerWatchdog
```

#### 3. Dry-Run Sandbox Testing
Test scripts with mock MQTT topic, payload, and argument inputs in an isolated sandbox without side effects:
```bash
# Test an existing configured script
mmq script test HighTempAlert --topic "sensors/room1/temperature" --payload '{"temp": 82.5}'

# Test inline code directly without saving or modifying broker configuration
mmq script test --lang starlark --topic "sensors/temp" --payload '{"temp": 90}' --code '
if msg != None:
    mqtt.publish("alerts/overheat", "ALARM", qos=1)
    log.warn("Overheat!")
'
```

#### 4. Execution Logs
Inspect circular execution log buffers for any script:
```bash
mmq script logs HighTempAlert
```

#### 5. Fetching Broker Documentation & AI Skills
Both Edge and MonsterMQ Core brokers authoritatively deliver documentation and AI skills for their supported runtimes via GraphQL:
```bash
# View Markdown API documentation for the broker's scripting runtime
mmq script docs
mmq script docs starlark

# Print ready-to-use AI Skill (SKILL.md) for coding assistants
mmq script skill
mmq script skill python

# Save the AI Skill to a local file
mmq script skill -o ./SKILL.md

# Install directly to local AI Agent skills directory (~/.gemini/antigravity/skills/monstermq-scripts/SKILL.md)
mmq script skill --install
```

### JSON Output & Automation
For automated processing or shell pipelines, put `--json` before the command:
```bash
mmq --json searchTopics "sensors/#"
```

---

## Schema-Driven Administration (Use This for Anything Without a Dedicated Command)

Every query and mutation of the connected broker is reachable without a dedicated command. Arguments are validated against the broker's live GraphQL schema (Full or Edge), so an agent never has to guess a JSON shape.

### Discover

```bash
mmq commands --json                 # machine-readable catalog: commands, options, broker support, exit codes, error shape
mmq schema                          # every query/mutation: path(args) -> return type
mmq schema user                     # operations whose path contains "user"
mmq schema user.createUser          # arguments, descriptions and all nested input types (JSON)
mmq schema CreateUserInput          # describe one type (input fields, defaults, enum values)
mmq api template user.createUser    # skeleton of the required arguments
```

### Call

```bash
mmq api users                                              # query by path
mmq api user.createUser --args @examples/api/create-user.json
mmq api user.createUser username=bob password=secret       # key=value; one input object may be given flat
mmq api mutation:publish topic=a/b payload=1               # prefix when a name exists as query and mutation
mmq api user.createUser --args @u.json --dry-run           # print generated GraphQL + variables only
mmq api session --select 'clientId connected' clientId=c1  # override the result selection
mmq gql '{ brokers { nodeId version } }'                   # raw GraphQL
mmq gql @query.graphql --vars '{"name":"x"}'
```

`--args` accepts inline JSON, `@file`, or `-` (stdin). A JSON array is mapped to the operation's single list argument (`publishBatch`). Output is JSON; password/secret/token fields are redacted unless `--reveal`. Mutations returning `success:false`, a non-empty `errors`, or `false` exit non-zero.

### Noun commands

Friendly aliases over the same engine (`mmq <noun> <verb>`; verbs are queries or mutation names, a prefix such as `create` resolves to `createUser`; a positional operand binds to the first argument):

| Noun | Query verbs | Mutations |
|---|---|---|
| `user` | `list` | `user.*` (createUser, updateUser, deleteUser, setPassword, ...) |
| `archiveGroup` | `list`, `get` | `archiveGroup.*` (create, update, delete, enable, disable, database connections) |
| `topicSchema` | `list`, `get`, `validate` | `topicSchemaPolicy.*` |
| `namespace` | `list`, `get`, `match` | `topicNamespace.*` |
| `flow` | `classes`, `instances`, `nodeTypes` | `flow.*` |
| `kafka` | `consumerGroups`, `messages`, `offsets` | - |
| `broker` | `list`, `get` | - |
| `dataCatalog` (Core) | `types`, `type`, `instances`, `instance`, `relations`, `infer` | `dataCatalog.*` (saveType, deleteType, saveInstance, deleteInstance, saveRelation, deleteRelation, exportCatalog, importCatalog) |
| `genai` / `ai` | `providers` + fields of the `genai` group (generate, analyzeTopics, proposeDataCatalog) | - |
| `uns` (Edge only, WinCC OA UNS Modeler) | `list`, `schema`, `prompt`, `validate` | `unsModeler.*` (create, update, delete, toggle, saveDraft, apply, createInWinCCOa, importProposal, startAiRun, cancelAiRun) |
| `virtualTopicTree` / `vtt` | `list`, `get`, `validate`, `preview` | `save`, `delete` (ungrouped; `save` takes `expectedRevision` for optimistic locking) |
| `redfish` | `list`, `get`, `sensors` | `save`, `delete`, `toggle` |
| `rtspCamera` / `camera` (Edge only) | `list`, `get` | `rtspCamera.*` (create, update, delete, start, stop, toggle, triggerSnapshot) |
| `peerLink` | `status` | - |
| `winccoa` (Edge only, inside WCCOAmmq) | fields of the `winccoa` browse group (`mmq schema winccoa`) | - |
| `agent`, `mcpServer`, `genAiProvider` | `list`, `get` | `agent.*`, `mcpServer.*`, `genAiProvider.*` |

Availability (enforced by tests against the Core and Edge source schemas; `mmq commands --json` carries it per command): **Core only** - `topicSchema`, `namespace`, `flow`, `agent`, `mcpServer`, `dataCatalog`; **Edge only** - `uns`, `winccoa`, `rtspCamera`; **both** - `user`, `archiveGroup`, `kafka`, `broker`, `genAiProvider`, `genai`, `peerLink`, `redfish`, `virtualTopicTree`.

### UNS Modeler (Edge, WinCC OA)

Maps WinCC OA datapoints into the UNS (`WINCCOA_TO_UNS`) or onboards MQTT topics into new WinCC OA datapoints (`MQTT_TO_WINCCOA`). Every step is available through `mmq uns`:

```bash
mmq --json uns schema                        # JSON Schema of the mapping document (add direction=WINCCOA_TO_UNS for the prompt view)
mmq --json uns prompt direction=MQTT_TO_WINCCOA   # built-in AI system prompt of a direction
mmq --json uns create --args '{"input":{"name":"line1","config":{"direction":"WINCCOA_TO_UNS","unsRoot":"plant/line1"}}}' --dry-run
mmq --json uns saveDraft name=line1 document=@mapping.json   # store the draft (validated, issues returned)
mmq --json uns validate name=line1 document=@mapping.json    # validate without storing
mmq --json uns apply name=line1              # validate the draft and apply it when it has no errors
mmq --json uns createInWinCCOa name=line1    # MQTT_TO_WINCCOA: create types/datapoints in WinCC OA
mmq --json uns startAiRun --args '{"name":"line1","input":{"prompt":"Map all pumps"}}'   # AI proposes a full mapping as the draft
mmq --json uns toggle name=line1 enabled=true
mmq --json winccoa --help                    # WinCC OA browsing (only when the Edge broker runs inside WCCOAmmq)
```

Note: `document` arguments are strings holding the mapping JSON; use `uns schema` to learn the document shape before writing one. The checked-in `gql/edge.gql` snapshot may lag behind a running Edge broker; refresh it with `python make.py gql-fetch`. The CLI itself always follows the connected broker's live schema.

Also: `mmq publishBatch --args '[...]'` and `mmq purgeQueuedMessages clientId=<id>`. Run `mmq <noun> --help` for the verbs, `mmq schema <path>` for exact arguments. On the Edge broker the data catalog schema defines types only (no operations); the data catalog commands exist on Core. Operations absent on the connected broker (e.g. most of these on an Edge broker) fail with "not available on this broker".

### Contract for automation

- `--json` is accepted anywhere on the command line. On failure it prints `{"success":false,"error":"..."}` to stdout (and `Error: ...` to stderr).
- Exit codes: `0` success, `1` command failed (GraphQL error, `success:false`, I/O or validation), `2` invalid global options.
- `mmq skill [--install [dir]]` generates a SKILL.md from the catalog (default install: `~/.claude/skills/monstermq-cli/`), so the skill can never drift from the binary.
- Ready-to-edit inputs: `examples/devices/*.json`, `examples/devices/templates/*.json` (one skeleton per device type), `examples/api/*.json`.

---

## Maintenance & Skill Synchronization

> **MANDATORY**: Whenever a subcommand, flag, GraphQL query, or feature capability is added, modified, or deprecated in `mmq`, this `SKILL.md` file **MUST** be updated to reflect the change.

## GraphQL Schemas and Comparison

The `gql/` directory maintains GraphQL SDL schemas (`main.gql` and `edge.gql`) fetched from running brokers, along with fetch and comparison tools:

```bash
# Fetch schemas from running brokers (and optionally compare)
python make.py gql-fetch --compare

# Compare schemas with the Go tool
python make.py gql-compare
# or summary only
python make.py gql-compare -summary
```

CLI device tests (`cli/devices_test.go`) read `main.gql` and `edge.gql` directly from `gql/` to validate actual queries, mutations, and variables without external checkout dependencies. `MMQ_CONTRACT_DIR` remains supported for optional test-only custom schema overrides.

Runtime `device schema` uses live introspection. GraphQL `#` comments are not introspection descriptions; use triple-quoted descriptions for AI-visible help. Session removal uses `session.removeSessions.results`.
