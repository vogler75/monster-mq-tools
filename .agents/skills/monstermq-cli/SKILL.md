---
name: monstermq-cli
description: Comprehensive operational guide and CLI reference for mmqcli (MonsterMQ CLI tool). Use this skill when interacting with, testing, or building features for Full MonsterMQ Broker or MonsterMQ Edge Broker instances.
---

# MonsterMQ CLI (`mmqcli`) Skill Guide

This skill provides operational instructions and command references for using `mmqcli` to interact with MonsterMQ Broker and MonsterMQ Edge Broker GraphQL endpoints.

---

## Quick Reference & Binary Location

- **Binary Location**: `cli/bin/mmqcli` (built via `cd cli && ./build.sh` or `cd cli && go build -o bin/mmqcli .`)
- **Default Endpoint**: `http://localhost:4000/graphql`
- **Configuration Sources**: CLI flags (`--url`, `--user`, `--pass`, `--token`), environment variables (`MQ_URL`, `MQ_USER`, `MQ_PASS`, `MQ_TOKEN`), or `.env` files.

---

## Operational Workflows

### 1. Endpoint Discovery & Feature Inspection
Before running complex queries against a target MonsterMQ node, inspect its enabled features to determine whether it is a central **Full Broker** or an **Edge Broker**:

```bash
mmqcli --url <endpoint-url> features
```

- **Full Broker**: Supports multi-group archives (`list-archives`), historical message logs (`query-history`), and TSDB aggregations (`query-aggregated`).
- **Edge Broker**: Focuses on real-time topic value reading/writing, edge device configuration sync, and MQTT client toggling.

---

## Command Reference

### 1:1 GraphQL Commands
`mmqcli` maps 1:1 to MonsterMQ GraphQL functions:
- **`searchTopics [pattern]`**: Search active topics (globs `*`, SQL `%`, MQTT `#`)
- **`currentValue <topic>`**: Get current or retained value for a single topic
- **`currentValues <filter>`**: Get current values matching a topic filter
- **`retainedMessages [filter]`**: List retained messages matching a topic filter
- **`browseTopics [path]`**: Browse topic hierarchy level-by-level
- **`publish <topic> <payload>`**: Publish message payload (`--retain`, `--qos 0|1|2`)
- **`archivedMessages <topic>`**: Query historical time-series messages
- **`aggregatedMessages <topics...>`**: Query server-side time-series aggregations (`AVG`, `MIN`, `MAX`)
- **`archiveGroups`**: List all deployed archive storage groups
- **`archiveStats <group>`**: Get stats for an archive group
- **`systemLogs`**: View broker system log entries (`--last-minutes N`)
- **`sessions`**: List active MQTT client sessions
- **`session <clientId>`**: Inspect specific client session details
- **`currentUser`**: Get authenticated user and role info
- **`databaseConnections`**: List configured database connections
- **`hmis`**: List deployed HMI web dashboards
- **`exportHmiZip <name>`**: Export HMI dashboard package
- **`brokerConfig`**: List enabled broker features & capabilities

### Device & Edge Management
- **List Devices**: `mmqcli device list`
- **Export Config**: `mmqcli device download [device-name] [output.json]`
- **Import Config**: `mmqcli device upload <config.json>`
- **Enable/Disable Device or Edge MQTT Client**:
  - `mmqcli device enable <device-name>`
  - `mmqcli device disable <device-name>`

### JSON Output & Automation
For automated processing or shell pipelines, append `--json`:
```bash
mmqcli --json list-topics "sensors/#"
```

---

## Maintenance & Skill Synchronization

> **MANDATORY**: Whenever a subcommand, flag, GraphQL query, or feature capability is added, modified, or deprecated in `mmqcli`, this `SKILL.md` file **MUST** be updated to reflect the change.
