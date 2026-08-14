---
name: monstermq-cli
description: Comprehensive operational guide and CLI reference for mmq (MonsterMQ CLI tool). Use this skill when interacting with, testing, or building features for Full MonsterMQ Broker or MonsterMQ Edge Broker instances.
---

# MonsterMQ CLI (`mmq`) Skill Guide

This skill provides operational instructions and command references for using `mmq` to interact with MonsterMQ Broker and MonsterMQ Edge Broker GraphQL endpoints.

---

## Quick Reference & Binary Location

- **Binary Location**: `cli/bin/mmq` (built via `cd cli && ./build.sh` or `cd cli && go build -o bin/mmq .`)
- **Default Endpoint**: `http://localhost:4000/graphql`
- **Configuration Sources**:
  - CLI flags: `--url`, `--host`, `--port`, `--https`, `--user`, `--pass`, `--token`, `-i`
  - Environment variables: `MQ_URL`, `MQ_HOST`, `MQ_PORT`, `MQ_HTTPS`, `MQ_USER`, `MQ_PASS`, `MQ_TOKEN` (or `GRAPHQL_*` equivalents)
  - `.env` files

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

# Or explicitly with shell subcommand:
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
- **`clear`**: Clear the terminal screen.
- **`help [command]`**: Display REPL interactive help menu.
- **`exit`** / **`quit`** (or `Ctrl+D`): Exit the interactive shell.

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
- **`searchTopics [pattern]`**: Search active topics (globs `*`, SQL `%`, MQTT `#`)
- **`currentValue <topic>`**: Get current or retained value for a single topic
- **`currentValues <filter>`**: Get current values matching a topic filter
- **`retainedMessages [filter]`**: List retained messages matching a topic filter
- **`browseTopics [path]`**: Browse topic hierarchy level-by-level
- **`publish <topic> <payload>`**: Publish message payload (`--retain`, `--qos 0|1|2`)
- **`archivedMessages <topic> [archiveGroup]`**: Query historical time-series messages (defaults to last 60s window)
- **`aggregatedMessages <topics...>`**: Query server-side time-series aggregations (`AVG`, `MIN`, `MAX`)
- **`archiveGroups`**: List all deployed archive storage groups
- **`archiveStats <group>`**: Get stats for an archive group
- **`systemLogs`**: View broker system log entries (`--last-minutes N`)
- **`sessions`**: List active MQTT client sessions
- **`session <clientId>`**: Inspect specific client session details
- **`currentUser`**: Get authenticated user and admin status
- **`databaseConnections`**: List configured database connections
- **`hmis` / `hmi list`**: List deployed HMI web dashboards
- **`hmi create <name> [options]`**: Create a new HMI dashboard definition
- **`hmi remove <name...>`**: Delete and remove one or more deployed HMI dashboards
- **`exportHmiZip <name> [out] [--unzip]`** (or `hmi export`): Export deployed HMI package to binary zip file or extract to folder
- **`importHmiZip <file.zip|dir> [name] [--main]`** (or `hmi import`): Upload & deploy HMI dashboard from a zip package or local directory
- **`brokerConfig`**: List enabled broker features & capabilities

### Device & Edge Management
- **List Devices**: `mmq device list [type] [--type <type>]`
- **Export Config**: `mmq device download [device-name] [output.json]`
- **Import Config**: `mmq device upload <config.json>`
- **Enable/Disable Device or Edge MQTT Client**:
  - `mmq device enable <device-name>`
  - `mmq device disable <device-name>`

### JSON Output & Automation
For automated processing or shell pipelines, append `--json`:
```bash
mmq --json searchTopics "sensors/#"
```

---

## Maintenance & Skill Synchronization

> **MANDATORY**: Whenever a subcommand, flag, GraphQL query, or feature capability is added, modified, or deprecated in `mmq`, this `SKILL.md` file **MUST** be updated to reflect the change.
