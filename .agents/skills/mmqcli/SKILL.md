---
name: mmqcli
description: Comprehensive operational guide and CLI reference for mmqcli (MonsterMQ CLI tool). Use this skill when interacting with, testing, or building features for Full MonsterMQ Broker or MonsterMQ Edge Broker instances.
---

# MonsterMQ CLI (`mmqcli`) Skill Guide

This skill provides operational instructions and command references for using `mmqcli` to interact with MonsterMQ Broker and MonsterMQ Edge Broker GraphQL endpoints.

---

## Quick Reference & Binary Location

- **Binary Location**: `bin/mmqcli` (built via `./build.sh` or `go build -o bin/mmqcli .`)
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

### Topic Operations
- **Get Retained/Current Topic Value**:
  ```bash
  mmqcli get-value <topic> [--archive-group GroupName]
  ```
- **Publish Payload**:
  ```bash
  mmqcli set-value <topic> <payload> [--retain] [--qos 0|1|2]
  ```
- **Search Active Topics**:
  ```bash
  mmqcli list-topics [pattern] [--limit N] [--archive-group GroupName]
  ```

### Historical & Time-Series Data (Full Broker / Configured Edge)
- **List Archive Groups**:
  ```bash
  mmqcli list-archives
  ```
- **Archive Statistics**:
  ```bash
  mmqcli archive-stats <group> [--start ISO_TIME] [--end ISO_TIME] [--last-seconds N]
  ```
- **Query Message History**:
  ```bash
  mmqcli query-history <topic> [--start ISO_TIME] [--end ISO_TIME] [--last-seconds N] [--limit N] [--archive-group GroupName]
  ```
- **Query Time-Series Aggregations**:
  ```bash
  mmqcli query-aggregated <topics...> [--interval ONE_MINUTE|FIVE_MINUTES|FIFTEEN_MINUTES|ONE_HOUR|ONE_DAY] [--functions AVG,MIN,MAX,COUNT] [--last-seconds N]
  ```

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
