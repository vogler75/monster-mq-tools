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
- **Manage Edge & Gateway Devices**: Import, export, enable, or disable device configurations dynamically.
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
Publish a message payload to a topic with optional retain and QoS flags.

```bash
mmq publish <topic> <payload> [--retain] [--qos 0|1|2]
```

*Example:*
```bash
mmq publish sensors/temp/room1 '{"temp": 22.5, "unit": "C"}' --retain --qos 1
```

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

Manage edge device, gateway, and subsystem configurations on both Full and Edge brokers.

#### `device list`
List all configured devices and edge nodes, with optional device type filtering.

```bash
mmq device list [type] [--type <type>]
```

*Examples:*
```bash
# List all devices
mmq device list

# Filter by type
mmq device list OPCUA_CLIENT
mmq device list --type MQTT_CLIENT
```

#### `device download`
Export device JSON configurations to standard output or a file.

```bash
mmq device download [device-name] [output-file.json]
```

#### `device upload`
Import or bulk update device configurations from a JSON file.

```bash
mmq device upload <config-file.json>
```

#### `device enable` / `device disable`
Enable or disable a configured device or edge MQTT client dynamically.

```bash
mmq device enable <device-name>
mmq device disable <device-name>
```

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
