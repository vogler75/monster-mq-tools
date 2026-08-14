# MonsterMQ Tools (`monster-mq-tools`)

`monster-mq-tools` is the central repository containing operational tools, CLI utilities, and developer applications for **MonsterMQ Full Broker** (main central instances) and **MonsterMQ Edge Broker** (lightweight edge nodes).

---

## Repository Overview

This repository provides command-line tools and utilities designed to simplify administration, data inspection, telemetry monitoring, device configuration management, and developer integration across all MonsterMQ broker deployments.

### Tools & Packages

| Directory | Tool | Description | Supported Brokers |
| :--- | :--- | :--- | :--- |
| [`/cli`](cli) | **MonsterMQ CLI (`mmq`)** | Go interactive REPL shell and command-line interface for GraphQL operations, real-time message publishing/subscribing, topic discovery, historical/TSDB metric querying, and device/client management. | Full Broker & Edge Broker |
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
To fetch and update both schemas from running broker instances:

```bash
./gql/fetch-schemas.sh
```

*(Optional parameters: `./gql/fetch-schemas.sh <main-url> <edge-url>` or via `MAIN_URL` and `EDGE_URL` environment variables).*

---

## License

MonsterMQ Tools is licensed under the terms included in the [LICENSE](LICENSE) file.
