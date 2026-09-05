# MonsterMQ CLI (`mmq`)

`mmq` is the official command-line interface for **MonsterMQ Broker** and **MonsterMQ Edge Broker**. It interacts seamlessly with MonsterMQ's GraphQL interface, providing administrative, operational, and data query capabilities for both full-scale enterprise broker deployments and lightweight edge broker nodes.

---

## Table of Contents

- [Overview](#overview)
- [Broker Support: Full vs Edge](#broker-support-full-vs-edge)
- [Installation & Building](#installation--building)
- [Configuration](#configuration)
  - [Global Options](#global-options)
  - [Environment Variables & `.env` File](#environment-variables--env-file)
- [Interactive REPL Shell](#interactive-repl-shell)
- [Authentication](#authentication)
- [Command Reference](#command-reference)
  - [Topic Management & Value Inspection](#topic-management--value-inspection)
  - [Historical Data & Archiving](#historical-data--archiving)
  - [Aggregated Time-Series Data](#aggregated-time-series-data)
  - [Device Configuration Management](#device-configuration-management)
  - [Broker & Feature Discovery](#broker--feature-discovery)
- [JSON Output Mode](#json-output-mode)
- [Examples & Common Workflows](#examples--common-workflows)
- [License](#license)

---

## Overview

`mmq` is a cross-platform Go utility designed to streamline operations across MonsterMQ deployments. Whether managing central data centers or remote edge nodes, `mmq` provides a unified CLI to:

- **Interactive Shell Session**: Connect once to a broker, maintain authentication, and execute commands continuously without restarting the CLI.
- **Publish & Inspect Messages**: Read current/retained topic values or publish payloads with QoS controls.
- **Search Topics**: Discover active topics matching wildcard patterns across archive groups.
- **Query Historical & Aggregated Metrics**: Extract time-series data, daily message counts, and historical logs.
- **Configure Devices**: Discover schemas, validate and apply settings, manage addresses and lifecycle, inspect status, and import/export backups.
- **Inspect Features**: Query broker nodes to inspect available and enabled features at runtime.

---

## Broker Support: Full vs Edge

`mmq` works out of the box with both deployment types:

| Feature / Capability | Full MonsterMQ Broker | MonsterMQ Edge Broker |
| :--- | :---: | :---: |
| **Interactive Shell / REPL** | ✅ Full | ✅ Full |
| **Topic Value Reading & Publishing** | ✅ Full | ✅ Full |
| **Topic Search & Wildcards** | ✅ Full | ✅ Full |
| **Device Configuration & Sync** | ✅ Full | ✅ Full (Edge MQTT Clients) |
| **Broker Feature Discovery** | ✅ Full (`features`) | ✅ Full (`features`) |
| **Historical Message Archiving** | ✅ Multi-Group Archives | ⚠️ Node-dependent / Local storage |
| **Time-Series Aggregations** | ✅ Advanced TSDB Aggregations | ⚠️ Enabled when edge persistence configured |

> **Tip**: Run `mmq features` against any endpoint to dynamically inspect the exact feature set enabled on that broker instance.

---

## Installation & Building

### Prerequisites
- [Go 1.20+](https://go.dev/doc/install) (for building from source)
- `bash` (for using `./build.sh`)

### Build from Source

You can build `mmq` using the included build script:

```bash
# Build native binary for current OS/architecture (output placed in bin/mmq)
./build.sh

# Cross-compile binaries for all supported platforms (Linux, macOS, Windows)
./build.sh --all

# Clean output directory
./build.sh --clean
```

Alternatively, compile directly with standard `go`:

```bash
go build -o bin/mmq .
```

---

## Configuration

`mmq` prioritizes configuration sources in the following order:
1. Command-line flags
2. Environment variables
3. `.env` file (defaults to `.env` in the current working directory)
4. Default values (`http://localhost:4000/graphql`)

### Global Options

| Flag | Short / Aliases | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--url` | - | `MQ_URL`, `GRAPHQL_URL` | `http://localhost:4000/graphql` | GraphQL endpoint URL |
| `--host` | - | `MQ_HOST`, `GRAPHQL_HOST` | `localhost` | Broker host / IP address |
| `--port` | - | `MQ_PORT`, `GRAPHQL_PORT` | `4000` | Broker port number |
| `--https` | - | `MQ_HTTPS`, `GRAPHQL_HTTPS` | `false` | Use HTTPS protocol instead of HTTP |
| `--user` | `--username` | `MQ_USER`, `GRAPHQL_USER` | - | Username for authentication |
| `--pass` | `--password` | `MQ_PASS`, `GRAPHQL_PASS` | - | Password for authentication |
| `--token` | - | `MQ_TOKEN`, `GRAPHQL_TOKEN` | - | JWT Bearer token (bypasses login) |
| `--env` | `--env-file` | - | `.env` | Path to custom `.env` file |
| `--json` | - | - | `false` | Output results in raw formatted JSON |
| `-i` | `--interactive` | - | - | Launch interactive REPL CLI session |
| `--help` | `-h` | - | - | Show CLI usage overview |

### Environment Variables & `.env` File

Create a `.env` file in your workspace or pass it via `--env <file>`:

```ini
MQ_HOST=192.168.1.50
MQ_PORT=4001
MQ_HTTPS=false
# Or explicit full URL:
# MQ_URL=http://192.168.1.50:4001/graphql
MQ_USER=admin
MQ_PASS=secretpassword
# Or pre-authenticated JWT token:
# MQ_TOKEN=eyJhbGciOi...
```

---

## Interactive REPL Shell

Instead of invoking `mmq` repeatedly for every command, you can launch a continuous interactive REPL shell with **Tab autocompletion** and **command history**:

```bash
# Start interactive shell (default when run without arguments)
mmq

# Or explicitly:
mmq shell
mmq --port 4001
mmq --host 192.168.1.50 --port 4001
```

### Welcome Banner & Status
```text
============================================================
  MonsterMQ Interactive CLI (mmq)
  Endpoint : http://localhost:4000/graphql
  Status   : Connected
  Auth     : admin (Admin)
============================================================
Type 'help' for commands, 'status' for broker info,
'connect <url|port>' to change endpoint, or 'exit' / 'quit' to exit.

mmq [localhost:4000]> 
```

### REPL Built-in Commands

| Command | Description | Example |
| :--- | :--- | :--- |
| `connect <url\|port>` | Switch / reconnect to a different broker endpoint | `connect 4001` or `connect http://192.168.1.10:4000/graphql` |
| `auth <user> <pass>` | Authenticate with username and password | `auth admin secret123` |
| `login <user> <pass>` | Alias for `auth` | `login admin secret123` |
| `token <jwt>` | Set or update JWT Bearer token | `token eyJhbGciOi...` |
| `status` | Display connection health, auth state, and features | `status` |
| `json [on\|off]` | Toggle or inspect JSON output mode | `json on` |
| `clear` | Clear terminal screen | `clear` |
| `help [cmd]` | Display interactive help menu | `help` |
| `exit` / `quit` | Exit the interactive session | `exit` |

---

## Authentication

`mmq` supports two authentication mechanisms:

1. **Username & Password Login**: When `--user` and `--pass` are provided (via flags, environment, `.env`, or the `auth` shell command), `mmq` automatically authenticates via the `login` GraphQL mutation.
2. **JWT Bearer Token**: If `--token` is specified (or set with `token <jwt>`), `mmq` attaches the token directly to the HTTP `Authorization: Bearer <token>` header, bypassing credential login queries.

---

## Command Reference

### Topic Management & Value Inspection

#### `currentValue`
Fetch the current or retained payload and metadata for a specific topic.

```bash
mmq currentValue <topic> [--archive-group GroupName]
```

*Example:*
```bash
mmq --port 4001 currentValue sensors/temp/room1
```

#### `publish`
Publish a message payload to a topic with optional retain and QoS flags. If payload is omitted, an empty message (`""`) is published (useful for clearing retained topics or triggering events).

```bash
mmq publish <topic> [payload] [--retain] [--qos 0|1|2]
```

*Examples:*
```bash
# Publish JSON telemetry:
mmq publish sensors/temp/room1 '{"temp": 22.5, "unit": "C"}' --retain --qos 1

# Clear a retained topic by publishing an empty payload:
mmq publish sensors/temp/room1 --retain
```

#### `subscribe`
Subscribe to real-time topic updates via WebSocket (`graphql-transport-ws`). Automatically derives the WebSocket URL (`ws://` or `wss://`) from the connected broker HTTP endpoint, supporting multiple simultaneous topic filters and Ctrl+C cancellation.

```bash
mmq subscribe <topic1> [topic2...] [options]
```

*Examples:*
```bash
# Subscribe to wildcard telemetry stream:
mmq subscribe "sensors/#" "factory/line1/+"

# Stream raw JSON lines:
mmq --json subscribe "sensors/#"
```

#### `monitor`
Interactive full-screen text-based live monitor table that tracks and displays current topic values in-place without cluttering scroll history. Supports terminal scrolling, sorting, and pause.

```bash
mmq monitor [topic-filters...]
# or: mmq subscribe "sensors/#" --monitor
```

*Interactive Controls:*
- `↑` / `↓` or `k` / `j`: Scroll rows up/down
- `PgUp` / `PgDn`: Scroll by page
- `Home` / `End` or `g` / `G`: Jump to top/bottom
- `s`: Cycle sort order (Alphabetical -> Timestamp -> Update Count)
- `p`: Pause/Resume live updates
- `c`: Clear tracked topics
- `q` / `Ctrl+C`: Exit monitor and restore terminal

#### `searchTopics`
Search active topics matching a pattern or wildcard across both the persistent archive store and live retained message store. Automatically handles glob patterns (`*Watt*`), SQL LIKE syntax (`%Watt%`), and MQTT topic filters (`sensors/#`).

```bash
mmq searchTopics [pattern] [--limit N] [--archive-group GroupName]
```

#### `currentValues`
Fetch current topic values matching an MQTT topic filter pattern.

```bash
mmq currentValues <topic-filter> [--limit N] [--archive-group Default]
```

#### `retainedMessages`
List all retained messages matching a topic filter pattern.

```bash
mmq retainedMessages [topic-filter] [--limit N]
```

#### `browseTopics`
Hierarchically browse topic levels.

```bash
mmq browseTopics [path] [--archive-group GroupName]
```

#### `sessions` / `session`
Manage connected MQTT client sessions.

```bash
mmq sessions
mmq session inspect <clientId>
mmq session remove <clientId...>
```

#### `systemLogs`
View broker system logs.

```bash
mmq systemLogs [--last-minutes N] [--limit N]
```

#### `hmis`
List deployed HMI web dashboards hosted by MonsterMQ.

```bash
mmq hmis
```

---

## Historical Data & Archiving

#### `archiveGroups`
List all deployed archive groups and their storage configurations.

```bash
mmq archiveGroups
```

#### `archiveStats`
Display min timestamps and daily message counts for an archive group.

```bash
mmq archiveStats <group> [--start ISO_TIME] [--end ISO_TIME] [--last-seconds N]
```

*Example:*
```bash
mmq archiveStats Default --last-seconds 86400
```

#### `archivedMessages`
Query historical messages matching topic filters over a time range. If no time range or `--last-seconds` is specified, defaults to the last 60 seconds. The archive group can be passed as a second positional argument or via `--archive-group` (default: `Default`).

```bash
mmq archivedMessages <topic> [archiveGroup] [--start ISO_TIME] [--end ISO_TIME] [--last-seconds N] [--limit N] [--archive-group GroupName]
```

*Examples:*
```bash
# Query last 60 seconds (default) from Default archive group
mmq archivedMessages "sensors/temp/room1"

# Query last 60 seconds from a custom archive group positionally
mmq archivedMessages "sensors/temp/room1" RawArchive

# Query last 1 hour with limit
mmq archivedMessages "sensors/temp/room1" --last-seconds 3600 --limit 20
```

---

## Aggregated Time-Series Data

#### `aggregatedMessages`
Query aggregated metrics (`AVG`, `MIN`, `MAX`, `COUNT`) across one or multiple topics over specified time intervals.

```bash
mmq aggregatedMessages <topics...> \
  [--interval ONE_MINUTE|FIVE_MINUTES|FIFTEEN_MINUTES|ONE_HOUR|ONE_DAY] \
  [--functions AVG,MIN,MAX,COUNT] \
  [--fields field1,field2] \
  [--start ISO_TIME] [--end ISO_TIME] [--last-seconds N] \
  [--archive-group GroupName]
```

*Example:*
```bash
mmq aggregatedMessages sensors/temp/room1 sensors/temp/room2 \
  --interval FIVE_MINUTES \
  --functions AVG,MAX \
  --last-seconds 3600
```

---

## Device Configuration Management

`mmq device` configures protocol connectors through their dedicated GraphQL APIs. Commands discover input fields and available operations from the target broker using introspection. Run `mmq device --help` for the command summary.

Supported adapters: MQTT, OPC UA, Kafka, WinCC OA, WinCC Unified, PLC4X, NATS, Redis, Neo4j, Telegram and i3X clients; OPC UA and Kafka servers; JDBC, InfluxDB and TimeBase loggers; Sparkplug B decoders. Type aliases ignore case, hyphens and underscores (`OPCUA_CLIENT`, `OPCUA-Client`, `opcua`). Availability depends on the target broker. HMI management remains under `mmq hmi`; this interface does not reconfigure broker listener/YAML settings or install missing protocol implementations.

### Discover the target

```bash
mmq --json device types
mmq --json device schema mqtt
mmq --json device schema opcua addAddress
mmq device template mqtt > mqtt.json
```

`types` lists the mutation operations actually exposed by the target and its enabled feature list. API presence does not guarantee that a feature is enabled on the assigned node; mutations enforce those checks. Edge exposes fewer APIs than Full Broker. New device commands require introspection. Name-based lookup and apply additionally require the `DeviceImportExport` feature, because they read existing configurations through `getDevices`.

`schema <type> [operation]` returns native GraphQL argument types, descriptions, defaults, nested input fields and enum choices. It defaults to the adapter's creation operation. `template` generates a minimal `{ "type": ..., "input": ... }` document with required fields. Fill in blank connection values and choose the appropriate `nodeId`; `*` follows the broker's assignment semantics and can mean all nodes. Optional settings are described by `schema`.

### Create and update

```bash
mmq --json device validate mqtt.json
mmq --json device apply mqtt.json --dry-run
mmq --json device apply mqtt.json
mmq --json device enable mqtt-example
mmq --json device status mqtt-example
```

Example apply document:

```json
{
  "type": "MQTT-Client",
  "input": {
    "name": "mqtt-example",
    "namespace": "external/mqtt",
    "nodeId": "*",
    "enabled": false,
    "config": {
      "brokerUrl": "tcp://localhost:1883",
      "clientId": "monstermq-example"
    }
  }
}
```

`apply` checks for an existing device by name, rejects type changes, and invokes the dedicated create/add or update mutation. New devices default to `enabled: false` unless explicitly supplied. For updates, supplied objects merge recursively with stored configuration; omitted values, credentials and enabled state are preserved. Arrays replace the corresponding array. Explicit null is sent to the broker, whose mutation semantics determine whether a nullable value can be cleared. Updates are read/merge/write operations, not atomic patches: avoid concurrent writers to the same device.

Connection settings for MQTT and OPC UA do not accept address arrays; their brokers manage addresses separately. Updates preserve existing mappings. Unsupported fields in user input fail validation rather than being silently dropped. Exported backup JSON is a different format; use `upload` for backups.

`validate` and `apply --dry-run` read the broker but never mutate it. Both show the intended operation, merged input and field changes, with password/secret/token/private-key fields redacted. Validation checks required fields, scalar types, lists, enum values and unknown fields against the target schema. It does not test connections, validate certificates, or replace broker-side semantic/range checks. There is no transaction or automatic rollback. Do not use these outputs as backup files. `[REDACTED]` placeholders are rejected as input; omit those fields when updating.

All file-input commands accept `-` for stdin:

```bash
cat mqtt.json | mmq --json device apply -
```

### Address mappings and OPC UA browsing

```bash
mmq --json device address list mqtt-example
mmq --json device schema mqtt addAddress
mmq --json device address add mqtt-example examples/devices/mqtt-address.json
mmq --json device address update mqtt-example 'sensors/#' examples/devices/mqtt-address.json
mmq --json device address delete mqtt-example 'sensors/#'

mmq --json device apply examples/devices/opcua.json
mmq --json device enable opcua-example
mmq --json device browse opcua-example
mmq --json device browse opcua-example 'ns=2;s=MyDevice'
mmq --json device address add opcua-example examples/devices/opcua-address.json
```

Address files contain the native address input object. The update/delete key is the broker's identifier, e.g. MQTT `remoteTopic`, OPC UA `address`, WinCC OA `query`, Unified `topic`, or i3X `elementId`. Operations not present in the target schema are rejected. OPC UA currently exposes add/delete, but no updateAddress operation; change a mapping through explicit delete/add operations. Browse uses the configured OPC UA client name and defaults to Objects (`i=85`). Edit the example endpoints, node assignments and addresses for your installation before applying them.

### Lifecycle, status and native operations

```bash
mmq --json device get mqtt-example
mmq --json device disable mqtt-example
mmq --json device delete mqtt-example
mmq --json device status opcua-example --wait --timeout 30s
mmq --json device enable opcua-example --wait --timeout 30s
```

Enable/disable use type-specific toggle APIs, never backup import. `get` redacts credential fields. `status` distinguishes available configuration state from runtime connectivity using `runtimeKnown`. `--wait` polls for connected=true (or connected=false for disable), and requires a runtime connectivity API. If unavailable, it fails before a lifecycle/apply mutation. In particular, the current Full/Edge MQTT device APIs do not expose that connectivity Boolean; inspect status and verify topic values instead. Apply with `--wait` requires `enabled: true`. A timeout does not undo a completed mutation. `--timeout` requires `--wait`, defaults to 30 seconds and accepts positive Go durations such as `5s` or `1m`; `--wait` and `--dry-run` cannot be combined.

For additional operations such as start/stop, reassignment, certificates or decoder rules, use the native mutation interface:

```bash
mmq --json device schema mqtt reassign
# reassign.json: {"name":"mqtt-example","nodeId":"node2"}
mmq --json device call mqtt reassign reassign.json --dry-run
mmq --json device call mqtt reassign reassign.json
```

`call <type> <operation> <args.json|->` validates the complete native argument object and checks mutation results. It does not perform apply's merge or default-to-disabled behavior; native API defaults apply. Use `schema` to discover arguments for operations whose signatures differ from the convenience commands.

### Backups and automation

```bash
mmq --json device list --type MQTT_CLIENT
mmq device download mqtt-example backup.json
mmq --json device upload backup.json
```

`list [type] [--type <type>]` filters configured devices. `download [name] [file.json]` exports raw configuration, including credentials. Newly created export files use owner-only permissions. `upload <file.json|->` accepts a single backup object or an array; both brokers force imported devices to disabled. Use dedicated enable commands afterwards. Import can partially succeed; any failed item results in a nonzero exit.

New configuration commands emit JSON in both normal and `--json` mode. With `--json`, failures produce `{ "success": false, "error": "..." }` on stdout and a nonzero process exit, with diagnostics on stderr. Place global options before `device`. Unknown options and missing arguments fail. AI workflow: discover types/schema → prepare input → validate/dry-run → apply → configure addresses → enable → check runtime state and topic values.

---

## Broker & Feature Discovery

#### `features` / `brokerConfig`
Query the connected broker instance (Full or Edge) to list all active feature flags and capabilities.

```bash
mmq features
```

#### `currentUser`
Inspect authenticated user and admin permissions.

```bash
mmq currentUser
```

#### `databaseConnections`
List configured database connections.

```bash
mmq databaseConnections
```

#### `hmis` / `hmi list`
List deployed HMI web dashboards with names, paths, and status.

```bash
mmq hmis
# or: mmq hmi list
```

#### `hmi create`
Create a new HMI web dashboard definition.

```bash
mmq hmi create <name> [--path /path] [--title "Title"] [--main]
```

#### `hmi remove`
Delete and remove one or more deployed HMI dashboards.

```bash
mmq hmi remove <name1> [name2...]
```

#### `exportHmiZip` / `hmi export`
Export deployed HMI package as a binary zip file or extract directly into a target folder using `--unzip`.

```bash
mmq exportHmiZip <dashboard-name> [output-file.zip]
mmq exportHmiZip <dashboard-name> [target-directory] --unzip
```

*Examples:*
```bash
# Save as zip archive:
mmq exportHmiZip FactoryOverview
# ✓ Exported HMI dashboard 'FactoryOverview' to 'FactoryOverview.zip' (45210 bytes)

# Extract directly to folder:
mmq exportHmiZip FactoryOverview ./src/hmi --unzip
# ✓ Exported and unzipped HMI dashboard 'FactoryOverview' into './src/hmi/' (45210 bytes archive)
```

#### `importHmiZip` / `hmi import`
Upload and deploy an HMI web dashboard from a binary zip file or local directory (automatically zipped on upload).

```bash
mmq importHmiZip <file.zip|directory> [dashboard-name] [--main]
```

*Examples:*
```bash
# Upload a zip archive:
mmq importHmiZip ./dist/FactoryOverview.zip

# Upload a directory directly (automatically zipped and deployed):
mmq importHmiZip ./src/hmi FactoryOverview --main
```

---

## JSON Output Mode

Append `--json` to any command (or toggle `json on` in the interactive shell) to receive raw JSON formatted output, ideal for scripting and `jq` filtering:

```bash
mmq --json device list | jq '.[] | select(.enabled == true)'
```

---

## Examples & Common Workflows

### 1. Interactive Session
```bash
# Start interactive shell against local or remote broker
mmq --port 4001

# Within the shell prompt:
mmq [localhost:4001]> features
mmq [localhost:4001]> searchTopics "sensors/#"
mmq [localhost:4001]> currentValue sensors/temp/room1
mmq [localhost:4001]> publish sensors/temp/room1 '{"temp": 23.1}' --retain
mmq [localhost:4001]> exit
```

### 2. Monitoring Edge Broker Telemetry
```bash
# Connect to an edge broker node on local network
mmq --host 192.168.1.50 --port 4001 currentValue "edge/gateway/status"
```

### 3. Backfilling / Deploying Device Configurations
```bash
# Download device configuration from central full broker
mmq --url http://central-broker:4000/graphql device download SensorNode1 config.json

# Upload device configuration to an edge broker node
mmq --port 4001 device upload config.json
```

### 4. Historical Telemetry Audit
```bash
# Query hourly average temperatures over the last 24 hours (86400 seconds)
mmq aggregatedMessages "factory/floor1/temp" --interval ONE_HOUR --functions AVG --last-seconds 86400
```

---

## License

MonsterMQ CLI is licensed under the terms included in the [LICENSE](file:///Users/vogler/Workspace/monster/cli/LICENSE) file.

## Keeping the CLI aligned with GraphQL

The tools-owned generator reads the actual Full/Edge broker schemas and validates
complete literal CLI operations and the device adapter registry. Runtime
`device schema` still uses live introspection.

There are no stored schema copies in the CLI repository. Device tests generate
contracts once per test run in a temporary directory from sibling `main` and
`edge` source checkouts, then validate their actual requests and variables against
those contracts. Temporary files are removed after loading. These tests require
both sibling checkouts, or an externally generated contract directory:

```bash
# From tools; use any output directory outside the repository.
./scripts/graphql-contract.sh --edge-root ../edge --cli-root . --output-dir /tmp/mmq-contract
(cd cli && MMQ_CONTRACT_DIR=/tmp/mmq-contract go test ./...)
```

`MMQ_CONTRACT_DIR` is a test-only environment variable pointing to a directory
containing `main/` and `edge/`, each with `schema.graphql` and `introspection.json`.
With sibling checkouts it may be omitted. Missing contracts or generation errors
fail the tests; they are not silently skipped. CI generates contracts under the
runner's temporary directory and passes that path to the tests.

The generator's default output is `main/doc/graphql/`, with separate `main/` and
`edge/` subdirectories. `--output-dir` redirects those artifacts; `--check` verifies
them without writing. `--cli-root` validates the CLI but writes no files into it.
Session removal uses `session.removeSessions.results`. New groups and behavior
changes still require adapter/workflow changes and integration tests.
