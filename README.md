# MonsterMQ Tools (`monster-mq-tools`)

`monster-mq-tools` is the central repository containing operational tools, CLI utilities, and developer applications for **MonsterMQ Full Broker** (main central instances) and **MonsterMQ Edge Broker** (lightweight edge nodes).

---

## Repository Overview

This repository provides command-line tools and utilities designed to simplify administration, data inspection, telemetry monitoring, device configuration management, and developer integration across all MonsterMQ broker deployments.

### Tools & Packages

| Directory | Tool | Description | Supported Brokers |
| :--- | :--- | :--- | :--- |
| [`/cli`](cli) | **MonsterMQ CLI (`mmqcli`)** | Go command-line interface for GraphQL operations, real-time message publishing/subscribing, topic discovery, historical/TSDB metric querying, and device/client management. | Full Broker & Edge Broker |

---

## Quick Start: MonsterMQ CLI (`mmqcli`)

The primary CLI tool is located in the [`cli/`](cli) directory.

### Build from Source

```bash
# Navigate to the CLI directory
cd cli

# Build native binary for host OS (output placed in cli/bin/mmqcli)
./build.sh

# Cross-compile binaries for Linux, macOS, and Windows
./build.sh --all
```

### Usage Examples

```bash
# Inspect enabled features on any broker (Full or Edge)
./bin/mmqcli --url http://localhost:4000/graphql features

# Publish a retained topic value
./bin/mmqcli set-value sensors/temp/room1 '{"temp": 22.5}' --retain

# Query time-series metric aggregations over the last hour
./bin/mmqcli query-aggregated sensors/temp/room1 --interval FIVE_MINUTES --functions AVG --last-seconds 3600

# Manage and list configured devices/subsystems
./bin/mmqcli device list
```

For full CLI documentation, global flags, environment configuration, and detailed command syntax, see [`cli/README.md`](cli/README.md).

---

## Agent Skills & Guidelines

This repository includes pre-packaged agent instructions and skills located under [`.agents/skills/`](.agents/skills/):
- **[`monstermq-cli`](.agents/skills/monstermq-cli/SKILL.md)**: Operational guide and CLI reference for `mmqcli`.
- **[`monstermq-hmi-builder`](.agents/skills/monstermq-hmi-builder/SKILL.md)**: Architecture patterns and guidelines for creating HTML/JS HMI screens and industrial dashboards hosted by MonsterMQ Edge.
- **[`monstermq-graphql`](.agents/skills/monstermq-graphql/SKILL.md)**: Data GraphQL API guide for querying topic values, publishing messages, inspecting archive groups, history data, time-series aggregations, and WebSockets.

Agent guidelines and contribution rules are detailed in [`AGENTS.md`](AGENTS.md).

---

## License

MonsterMQ Tools is licensed under the terms included in the [LICENSE](LICENSE) file.
