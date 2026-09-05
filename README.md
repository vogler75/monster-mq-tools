# MonsterMQ Tools (`monster-mq-tools`)

`monster-mq-tools` is the central repository containing operational tools, CLI utilities, and developer applications for **MonsterMQ Full Broker** (main central instances) and **MonsterMQ Edge Broker** (lightweight edge nodes).

---

## Repository Overview

This repository provides command-line tools and utilities designed to simplify administration, data inspection, telemetry monitoring, device configuration management, and developer integration across all MonsterMQ broker deployments.

### Tools & Packages

| Directory | Tool | Description | Supported Brokers |
| :--- | :--- | :--- | :--- |
| [`/cli`](cli) | **MonsterMQ CLI (`mmq`)** | Go interactive REPL shell and command-line interface for GraphQL operations, real-time message publishing/subscribing, topic discovery, historical/TSDB metric querying, and device/client management. | Full Broker & Edge Broker |
| [`/i3x`](i3x) | **i3X CLI (`i3x`)** | Go interactive REPL shell and command-line tool for i3X 1.0 API specification (Industrial Information Interface eXchange), exploring namespaces/types/objects, querying/writing values, historical telemetry, and SSE live subscriptions. | i3X 1.0 Compliant Brokers & Servers |
| [`/mbp`](mbp) | **Build Pipeline (`mbp`)** | Go interactive Terminal UI (TUI) and headless CLI for inspecting git/build statuses, building, observing real-time logs, and publishing all MonsterMQ components (`main`, `edge`, `dashboard`, `explorer`, `tools`). | Ecosystem Orchestrator |
| [`/hmi`](hmi) | **Edge HMI Dashboards** | Standalone web HMIs and industrial dashboard applications hosted and served directly by MonsterMQ Edge brokers. | Edge Broker |

---

## Quick Start: MonsterMQ CLI (`mmq`)

The primary CLI tool is located in the [`cli/`](cli) directory.

### Build from Source

```bash
# Navigate to the CLI directory
cd cli

# Build native binary for host OS (output placed in cli/bin/mmq)
./build.sh

# Cross-compile binaries for Linux, macOS, and Windows
./build.sh --all
```

### Usage Examples

#### 1. Interactive REPL Shell (Continuous Session)
```bash
# Launch interactive shell connected to default or custom broker
./bin/mmq --url http://localhost:4000/graphql

# Execute commands continuously without re-running the binary:
mmq [localhost:4000]> features
mmq [localhost:4000]> searchTopics "*"
mmq [localhost:4000]> currentValue sensors/temp/room1
mmq [localhost:4000]> publish sensors/temp/room1 '{"temp": 22.5}' --retain
mmq [localhost:4000]> exit
```

#### 2. One-Shot Command Execution
```bash
# Inspect enabled features on any broker (Full or Edge)
./bin/mmq --url http://localhost:4000/graphql features

# Publish a retained topic value
./bin/mmq publish sensors/temp/room1 '{"temp": 22.5}' --retain

# Subscribe to real-time topic updates via WebSocket
./bin/mmq subscribe "sensors/#" "factory/line1/+"

# Interactive live dashboard topic monitor
./bin/mmq monitor "sensors/#"

# Query time-series metric aggregations over the last hour
./bin/mmq aggregatedMessages sensors/temp/room1 --interval FIVE_MINUTES --functions AVG --last-seconds 3600

# Manage and list configured devices/subsystems
./bin/mmq device list
```

For full CLI documentation, global flags, environment configuration, and detailed command syntax, see [`cli/README.md`](cli/README.md).

---

## Agent Skills & Guidelines

This repository includes pre-packaged agent instructions and skills located under [`.agents/skills/`](.agents/skills/):
- **[`monstermq-cli`](.agents/skills/monstermq-cli/SKILL.md)**: Operational guide and CLI reference for `mmq`.
- **[`monstermq-i3x`](.agents/skills/monstermq-i3x/SKILL.md)**: Operational guide and CLI reference for `i3x`.
- **[`monstermq-hmi-builder`](.agents/skills/monstermq-hmi-builder/SKILL.md)**: Architecture patterns and guidelines for creating HTML/JS HMI screens and industrial dashboards hosted by MonsterMQ Edge.
- **[`monstermq-graphql`](.agents/skills/monstermq-graphql/SKILL.md)**: Data GraphQL API guide for querying topic values, publishing messages, inspecting archive groups, history data, time-series aggregations, and WebSockets.

Agent guidelines and contribution rules are detailed in [`AGENTS.md`](AGENTS.md).

For GraphQL schema parity and Edge broker adaptation specifications, see [`EDGE_BROKER_GRAPHQL_ADAPTATIONS.md`](EDGE_BROKER_GRAPHQL_ADAPTATIONS.md).

---

## GraphQL Schemas & Utilities

Current GraphQL SDL schemas are stored under [`gql/`](gql/):
- **[`gql/main.gql`](gql/main.gql)**: Main Broker GraphQL schema (`http://localhost:4000/graphql`).
- **[`gql/edge.gql`](gql/edge.gql)**: Edge Broker GraphQL schema (`http://localhost:4001/graphql`).

### Re-fetching Schemas
To fetch and update schemas from running broker instances:

```bash
# Fetch both Main and Edge schemas
./gql/fetch-schemas.sh

# Fetch only Main Broker schema
./gql/fetch-schemas.sh -m

# Fetch only Edge Broker schema
./gql/fetch-schemas.sh -e
```

*(Run `./gql/fetch-schemas.sh -h` for full usage, custom URLs, and environment variable options).*

---

## License

MonsterMQ Tools is licensed under the terms included in the [LICENSE](LICENSE) file.

### AI-driven device configuration

The MonsterMQ CLI now discovers device APIs from the target broker and configures
MQTT, OPC UA, Kafka, WinCC OA/Unified, PLC4X, NATS, Redis, Neo4j, Telegram, i3X,
OPC UA/Kafka servers, JDBC/InfluxDB/TimeBase loggers and Sparkplug B decoders where
those APIs are available. Edge and Full Broker capabilities differ.

```bash
mmq --json device types
mmq device template mqtt > mqtt.json
# Fill in connection settings and the node assignment.
mmq --json device validate mqtt.json
mmq --json device apply mqtt.json --dry-run
mmq --json device apply mqtt.json
mmq --json device enable my-device
mmq --json device status my-device
```

Use `device schema`, `device address`, `device browse`, and `device call` for
protocol-specific settings and operations. New devices default to disabled;
updates preserve omitted settings. Import/export remains a separate backup
workflow, and imports always disable devices. Failed operations, including partial
imports, return nonzero exits. Runtime waiting is available only when the target
API exposes connectivity; enabled state alone is not connection confirmation.

See the [complete device command reference](cli/README.md#device-configuration-management)
and [MQTT/OPC UA example files](cli/examples/devices). No broker changes are required
for APIs already exposed by the target. Name-based configuration requires the
`DeviceImportExport` feature, and schema discovery requires introspection.

### GraphQL schemas and comparison

The `gql/` directory contains GraphQL SDL schemas (`main.gql` and `edge.gql`) fetched from running brokers, along with tooling to fetch and compare them:

```bash
# Fetch schemas from running brokers (and optionally compare)
./gql/fetch-schemas.sh --compare

# Compare schemas (main.gql vs edge.gql)
./gql/compare-schemas.sh
# or with summary only
(cd gql && go run . -summary)
```

Device tests in `cli/` validate actual GraphQL requests and schema introspection directly against `gql/main.gql` and `gql/edge.gql` (or `MMQ_CONTRACT_DIR` if overridden). Run unit tests via `(cd cli && go test ./...)`.

---

## Quick Start: Build Pipeline & Orchestrator (`mbp`)

Located in [`mbp/`](mbp). Provides an interactive Terminal UI and headless CLI to inspect, compile, build, observe, and publish all MonsterMQ components (`main`, `edge`, `dashboard`, `explorer`, `tools`).

```bash
# Build the mbp tool
(cd mbp && ./build.sh)

# Launch interactive Terminal UI
./mbp/bin/mbp

# Print component status overview
./mbp/bin/mbp status

# Pull latest git updates
./mbp/bin/mbp pull explorer
./mbp/bin/mbp pull all

# Headlessly build a component or all components
./mbp/bin/mbp build edge
./mbp/bin/mbp build all
```

