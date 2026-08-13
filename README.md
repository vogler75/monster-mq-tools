# MonsterMQ CLI (`mmqcli`)

`mmqcli` is the official command-line interface for **MonsterMQ Broker** and **MonsterMQ Edge Broker**. It interacts seamlessly with MonsterMQ's GraphQL interface, providing administrative, operational, and data query capabilities for both full-scale enterprise broker deployments and lightweight edge broker nodes.

---

## Table of Contents

- [Overview](#overview)
- [Broker Support: Full vs Edge](#broker-support-full-vs-edge)
- [Installation & Building](#installation--building)
- [Configuration](#configuration)
  - [Global Options](#global-options)
  - [Environment Variables & `.env` File](#environment-variables--env-file)
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

`mmqcli` is a cross-platform Go utility designed to streamline operations across MonsterMQ deployments. Whether managing central data centers or remote edge nodes, `mmqcli` provides a unified CLI to:

- **Publish & Inspect Messages**: Read current/retained topic values or publish payloads with QoS controls.
- **Search Topics**: Discover active topics matching wildcard patterns across archive groups.
- **Query Historical & Aggregated Metrics**: Extract time-series data, daily message counts, and historical logs.
- **Manage Edge & Gateway Devices**: Import, export, enable, or disable device configurations and MQTT clients dynamically.
- **Inspect Features**: Query broker nodes to inspect available and enabled features at runtime.

---

## Broker Support: Full vs Edge

`mmqcli` works out of the box with both deployment types:

| Feature / Capability | Full MonsterMQ Broker | MonsterMQ Edge Broker |
| :--- | :---: | :---: |
| **Topic Value Reading & Publishing** | ✅ Full | ✅ Full |
| **Topic Search & Wildcards** | ✅ Full | ✅ Full |
| **Device Configuration & Sync** | ✅ Full | ✅ Full (Edge MQTT Clients) |
| **Broker Feature Discovery** | ✅ Full (`features`) | ✅ Full (`features`) |
| **Historical Message Archiving** | ✅ Multi-Group Archives | ⚠️ Node-dependent / Local storage |
| **Time-Series Aggregations** | ✅ Advanced TSDB Aggregations | ⚠️ Enabled when edge persistence configured |

> **Tip**: Run `mmqcli features` against any endpoint to dynamically inspect the exact feature set enabled on that broker instance.

---

## Installation & Building

### Prerequisites
- [Go 1.20+](https://go.dev/doc/install) (for building from source)
- `bash` (for using `./build.sh`)

### Build from Source

You can build `mmqcli` using the included build script:

```bash
# Build native binary for current OS/architecture (output placed in bin/mmqcli)
./build.sh

# Cross-compile binaries for all supported platforms (Linux, macOS, Windows)
./build.sh --all

# Clean output directory
./build.sh --clean
```

Alternatively, compile directly with standard `go`:

```bash
go build -o bin/mmqcli .
```

---

## Configuration

`mmqcli` prioritizes configuration sources in the following order:
1. Command-line flags
2. Environment variables
3. `.env` file (defaults to `.env` in the current working directory)
4. Default values (`http://localhost:4000/graphql`)

### Global Options

| Flag | Short / Aliases | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--url` | - | `MQ_URL`, `GRAPHQL_URL` | `http://localhost:4000/graphql` | GraphQL endpoint URL |
| `--user` | `--username` | `MQ_USER`, `GRAPHQL_USER` | - | Username for authentication |
| `--pass` | `--password` | `MQ_PASS`, `GRAPHQL_PASS` | - | Password for authentication |
| `--token` | - | `MQ_TOKEN`, `GRAPHQL_TOKEN` | - | JWT Bearer token (bypasses login) |
| `--env` | `--env-file` | - | `.env` | Path to custom `.env` file |
| `--json` | - | - | `false` | Output results in raw formatted JSON |
| `--help` | `-h` | - | - | Show CLI usage overview |

### Environment Variables & `.env` File

Create a `.env` file in your workspace or pass it via `--env <file>`:

```ini
MQ_URL=http://localhost:4000/graphql
MQ_USER=admin
MQ_PASS=secretpassword
# Or pre-authenticated JWT token:
# MQ_TOKEN=eyJhbGciOi...
```

---

## Authentication

`mmqcli` supports two authentication mechanisms:

1. **Username & Password Login**: When `--user` and `--pass` are provided (via flags, environment, or `.env`), `mmqcli` automatically authenticates via the `login` GraphQL mutation before executing subcommands.
2. **JWT Bearer Token**: If `--token` is specified, `mmqcli` attaches the token directly to the HTTP `Authorization: Bearer <token>` header, bypassing credential login queries.

---

## Command Reference

### Topic Management & Value Inspection

#### `get-value` (alias: `get`)
Fetch the current or retained payload and metadata for a specific topic.

```bash
mmqcli get-value <topic> [--archive-group GroupName]
```

*Example:*
```bash
mmqcli --url http://edge-node:4000/graphql get-value sensors/temp/room1
```

#### `set-value` (alias: `publish`)
Publish a message payload to a topic with optional retain and QoS flags.

```bash
mmqcli set-value <topic> <payload> [--retain] [--qos 0|1|2]
```

*Example:*
```bash
mmqcli set-value sensors/temp/room1 '{"temp": 22.5, "unit": "C"}' --retain --qos 1
```

#### `list-topics` (alias: `search-topics`)
Search active topics matching an MQTT pattern or wildcard.

```bash
mmqcli list-topics [pattern] [--limit N] [--archive-group GroupName]
```

*Example:*
```bash
mmqcli list-topics "sensors/#" --limit 50
```

---

### Historical Data & Archiving

#### `list-archives`
List all deployed archive groups and their storage configurations.

```bash
mmqcli list-archives
```

#### `archive-stats`
Display min timestamps and daily message counts for an archive group.

```bash
mmqcli archive-stats <group> [--start ISO_TIME] [--end ISO_TIME] [--last-seconds N]
```

*Example:*
```bash
mmqcli archive-stats Default --last-seconds 86400
```

#### `query-history` (alias: `history`)
Query historical messages matching topic filters over a time range.

```bash
mmqcli query-history <topic> [--start ISO_TIME] [--end ISO_TIME] [--last-seconds N] [--limit N] [--archive-group GroupName]
```

*Example:*
```bash
mmqcli query-history "sensors/temp/room1" --last-seconds 3600 --limit 20
```

---

### Aggregated Time-Series Data

#### `query-aggregated` (alias: `aggregated`)
Query aggregated metrics (`AVG`, `MIN`, `MAX`, `COUNT`) across one or multiple topics over specified time intervals.

```bash
mmqcli query-aggregated <topics...> \
  [--interval ONE_MINUTE|FIVE_MINUTES|FIFTEEN_MINUTES|ONE_HOUR|ONE_DAY] \
  [--functions AVG,MIN,MAX,COUNT] \
  [--fields field1,field2] \
  [--start ISO_TIME] [--end ISO_TIME] [--last-seconds N] \
  [--archive-group GroupName]
```

*Example:*
```bash
mmqcli query-aggregated sensors/temp/room1 sensors/temp/room2 \
  --interval FIVE_MINUTES \
  --functions AVG,MAX \
  --last-seconds 3600
```

---

### Device Configuration Management

Manage edge device, gateway, and subsystem configurations on both Full and Edge brokers.

#### `device list` (alias: `device ls`)
List all configured devices and edge nodes.

```bash
mmqcli device list
```

#### `device download` (alias: `device export`)
Export device JSON configurations to standard output or a file.

```bash
mmqcli device download [device-name] [output-file.json]
```

#### `device upload` (alias: `device import`)
Import or bulk update device configurations from a JSON file.

```bash
mmqcli device upload <config-file.json>
```

#### `device enable` / `device disable`
Enable or disable a configured device or edge MQTT client dynamically.

```bash
mmqcli device enable <device-name>
mmqcli device disable <device-name>
```

---

### Broker & Feature Discovery

#### `features` (aliases: `enabled-features`, `list-features`)
Query the connected broker instance (Full or Edge) to list all active feature flags and capabilities.

```bash
mmqcli features
```

---

## JSON Output Mode

Append `--json` to any command to receive raw JSON formatted output, ideal for scripting and `jq` filtering:

```bash
mmqcli --json device list | jq '.[] | select(.enabled == true)'
```

---

## Examples & Common Workflows

### 1. Monitoring Edge Broker Telemetry
```bash
# Connect to an edge broker node on local network
mmqcli --url http://192.168.1.50:4000/graphql get-value "edge/gateway/status"
```

### 2. Backfilling / Deploying Device Configurations
```bash
# Download device configuration from central full broker
mmqcli --url http://central-broker:4000/graphql device download SensorNode1 config.json

# Upload device configuration to an edge broker node
mmqcli --url http://edge-node:4000/graphql device upload config.json
```

### 3. Historical Telemetry Audit
```bash
# Query hourly average temperatures over the last 24 hours (86400 seconds)
mmqcli query-aggregated "factory/floor1/temp" --interval ONE_HOUR --functions AVG --last-seconds 86400
```

---

## License

MonsterMQ CLI is licensed under the terms included in the [LICENSE](file:///Users/vogler/Workspace/monster/cli/LICENSE) file.
