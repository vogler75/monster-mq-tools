# i3x CLI (`i3x`)

Official command-line tool and interactive REPL client for **i3X (Industrial Information Interface eXchange) 1.0 Specification** ([https://api.i3x.dev/v1/docs](https://api.i3x.dev/v1/docs)).

---

## Overview

`i3x` provides a fast, standalone CLI to connect to any i3X compliant industrial server (such as MonsterMQ Edge brokers or CESMII/ThinkIQ/Abelara i3X implementations) to explore namespaces, inspect object and relationship types, query and write object values, retrieve historical time-series telemetry, and manage live Server-Sent Events (SSE) subscriptions.

### Key Capabilities

- **100% i3X 1.0 API Coverage**: Full support for all 18 endpoints across Info, Explore, Query, Update, and Subscribe tags.
- **Interactive REPL Mode**: Continuous shell with TAB auto-completion, command history, persistent session state, and fast context switching.
- **Rich Output Formatting**: Beautiful ANSI tables, hierarchical trees, compact/indented JSON, CSV export, and raw modes.
- **High-Level Live Watcher (`watch`)**: One-command live streaming with automatic subscription creation, item registration, and graceful cleanup on exit.
- **Flexible Time Range Parsing**: Support for relative timestamps like `-15m`, `-1h`, `-24h`, `-7d`, or standard RFC 3339 timestamps.
- **Multi-Authentication Support**: Bearer tokens, API keys (`X-API-Key`), and custom HTTP headers.
- **Cross-Platform Single Binary**: Pre-compiled for Linux (amd64, arm64, armv7), macOS (Apple Silicon & Intel), and Windows.

---

## Installation & Build

### Build from Source

Requirements: Go 1.22+

```bash
# Navigate to the i3x directory
cd i3x

# Build native binary for host OS (placed in i3x/bin/i3x)
./build.sh

# Cross-compile for Linux, macOS, and Windows
./build.sh --all
```

---

## Quick Start

### 1. Interactive REPL Shell

Simply launch `i3x` without arguments to enter the interactive shell:

```bash
./bin/i3x --url https://api.i3x.dev
```

```text
=========================================================
           i3X Industrial API 1.0 Shell                  
=========================================================
Connected to: https://api.i3x.dev
Client ID:    i3x-cli-176321
Type help for command overview, exit or Ctrl+D to quit.

api.i3x.dev> info
api.i3x.dev> namespaces
api.i3x.dev> objects --format tree
api.i3x.dev> read pump-station
api.i3x.dev> write pump-station 42.5 --quality Good
api.i3x.dev> watch pump-station pump-101
api.i3x.dev> exit
```

### 2. One-Shot Commands

```bash
# Check server health and capabilities
i3x info

# List registered namespaces
i3x namespaces

# List object types
i3x types

# Query object schema by element ID
i3x types query work-center-type

# Browse objects in hierarchical tree format
i3x objects --format tree

# Read last known values of multiple objects
i3x read pump-station pump-101 sensor-001

# Write a new value
i3x write pump-station 42.5 --quality Good

# Query 1-hour telemetry history
i3x history pump-station --start -1h

# Watch real-time updates via live SSE stream
i3x watch pump-station pump-101
```

---

## Command Reference

### 1. Server Info & Health

| Command | Endpoint | Description |
| :--- | :--- | :--- |
| `i3x info` | `GET /v1/info` | Inspect spec version, server version, and query/update/stream capabilities |

### 2. Explore (Namespaces, Types, Objects)

| Command | Endpoint | Description |
| :--- | :--- | :--- |
| `i3x namespaces` | `GET /v1/namespaces` | List all registered namespaces |
| `i3x types [--namespace <uri>]` | `GET /v1/objecttypes` | List object type definitions, optionally filtered by namespace |
| `i3x types query <id...>` | `POST /v1/objecttypes/query` | Query object type schemas by element ID |
| `i3x rel-types [--namespace <uri>]` | `GET /v1/relationshiptypes` | List relationship types |
| `i3x rel-types query <id...>` | `POST /v1/relationshiptypes/query` | Query relationship types by element ID |
| `i3x objects [options]` | `GET /v1/objects` | List all object instances with filtering (`--type`, `--filter`, `--name`, `--parent`, `--root`, `--composition`) |
| `i3x objects query <id...> [--metadata]` | `POST /v1/objects/list` | Query object instances by element ID |
| `i3x related <id...> [--rel-type <type>]` | `POST /v1/objects/related` | Query related objects by relationship type |

### 3. Values (Query & Update)

| Command | Endpoint | Description |
| :--- | :--- | :--- |
| `i3x read <id...> [--depth <n>]` | `POST /v1/objects/value` | Read last known values. `depth=0` recurses child components |
| `i3x write <id> <val> [--quality <q>] [--timestamp <t>]` | `PUT /v1/objects/value` | Write current value for an object instance |
| `i3x write id1=val1 id2=val2` | `PUT /v1/objects/value` | Write multiple object values in batch |
| `i3x write --json '<json>'` | `PUT /v1/objects/value` | Write values using raw JSON payload |

### 4. History (Time-Series Telemetry)

| Command | Endpoint | Description |
| :--- | :--- | :--- |
| `i3x history <id...> [--start <t>] [--end <t>] [--depth <n>]` | `POST /v1/objects/history` | Query historical values over time range (e.g. `--start -2h`) |
| `i3x write-history <id> <val> [--quality <q>] [--timestamp <t>]` | `PUT /v1/objects/history` | Record historical data point |
| `i3x write-history --json '<json>'` | `PUT /v1/objects/history` | Batch record historical entries |

### 5. Subscriptions (SSE & Sync)

| Command | Endpoint | Description |
| :--- | :--- | :--- |
| `i3x sub create [--name <name>]` | `POST /v1/subscriptions` | Create a new subscription scoped to client ID |
| `i3x sub list <subId...>` | `POST /v1/subscriptions/list` | List active subscriptions and monitored objects |
| `i3x sub register <subId> <id...> [--depth <n>]` | `POST /v1/subscriptions/register` | Register element IDs to monitor |
| `i3x sub unregister <subId> <id...>` | `POST /v1/subscriptions/unregister` | Remove element IDs from subscription |
| `i3x sub sync <subId> [--ack-seq <n>]` | `POST /v1/subscriptions/sync` | Pull pending batches and acknowledge sequence number |
| `i3x sub stream <subId>` | `POST /v1/subscriptions/stream` | Open live Server-Sent Events (SSE) stream |
| `i3x sub delete <subId...>` | `POST /v1/subscriptions/delete` | Delete one or more subscriptions |

### 6. High-Level Live Watcher

```bash
# Watch multiple objects live. Automatically creates subscription, registers items, streams updates, and auto-deletes subscription on Ctrl+C.
i3x watch pump-station pump-101 sensor-001
```

---

## Global Options & Environment Variables

| Flag | Shorthand | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--url` | `-u` | `I3X_URL` | `https://api.i3x.dev` | i3X server base URL |
| `--client-id` | `-c` | `I3X_CLIENT_ID` | Hostname-based UUID | Client ID scoping subscriptions |
| `--token` | `-t` | `I3X_TOKEN` | `""` | Bearer token for HTTP Authorization header |
| `--api-key` | `-k` | `I3X_API_KEY` | `""` | API Key for `X-API-Key` header |
| `--header` | `-H` | - | `""` | Custom HTTP header (e.g. `-H "X-Custom: val"`) |
| `--format` | `-o` | `I3X_FORMAT` | `table` | Output format: `table`, `json`, `raw`, `csv`, `tree` |
| `--no-color` | - | `NO_COLOR` | `false` | Disable colored terminal output |
| `--verbose` | `-v` | - | `false` | Enable verbose HTTP request & response logging |
| `--insecure` | - | - | `false` | Skip TLS certificate verification |
| `--timeout` | - | - | `30s` | HTTP request timeout duration |

---

## Automation & Scripting Examples

### Export Objects to CSV
```bash
i3x objects --format csv > objects.csv
```

### JSON Pipelines with `jq`
```bash
# Extract value from JSON response
i3x read pump-station -o json | jq '.results[0].result.value'
```

### Continuous Polling via Sync
```bash
# Sync pending batches acknowledging up to sequence #10
i3x sub sync 522dfbf1-9cbd-4e69-91fe-ad00bb1f64ba --ack-seq 10
```

---

## Testing

Run the full automated test suite:

```bash
cd i3x
go test -v ./...
```
