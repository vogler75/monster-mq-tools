---
name: monstermq-i3x
description: Comprehensive operational guide and CLI reference for i3x (Industrial Information Interface eXchange 1.0 API). Use this skill when interacting with, querying, writing values to, or streaming live telemetry from i3X servers and MonsterMQ brokers.
---

# i3X CLI (`i3x`) Skill Guide

This skill provides operational instructions and command references for using the `i3x` CLI tool to interact with i3X 1.0 compliant servers and industrial endpoints.

---

## Quick Reference & Binary Location

- **Binary Location**: `i3x/bin/i3x` (built via `cd i3x && ./build.sh`)
- **Default Endpoint**: `https://api.i3x.dev` (or configured via `I3X_URL` / `-u <url>`)
- **API Spec Reference**: `https://api.i3x.dev/v1/docs` (i3X 1.0)
- **Configuration Sources**:
  - CLI flags: `-u, --url`, `-c, --client-id`, `-t, --token`, `-k, --api-key`, `-H, --header`, `-o, --format`
  - Environment variables: `I3X_URL`, `I3X_CLIENT_ID`, `I3X_TOKEN`, `I3X_API_KEY`, `I3X_FORMAT`, `NO_COLOR`
  - `.env` files in execution directory or parent directories

---

## Interactive Shell / REPL Mode

Launch the interactive REPL shell:

```bash
# Start interactive shell on default or custom server
./i3x/bin/i3x
./i3x/bin/i3x --url http://localhost:8080
```

Inside the interactive prompt (`i3x [host]>`):
- `info`: Check server status and capabilities.
- `namespaces`: List namespaces.
- `types [query <id>]`: List/query object types.
- `objects [--format tree]`: Browse object hierarchy.
- `read <id...>`: Query last known values.
- `write <id> <value>`: Write current value.
- `history <id...> [--start -1h]`: Query time-series telemetry.
- `sub create / sub register / sub sync / sub stream / sub delete`: Manage subscriptions.
- `watch <id...>`: Stream live telemetry.
- `use <subscriptionId>`: Set active subscription context.
- `format table|json|raw|csv|tree`: Switch output format live.
- `help`: Show interactive help.

---

## Common CLI Workflows

### 1. Server Capabilities & Health
```bash
i3x --url https://api.i3x.dev info
```

### 2. Namespace & Type Exploration
```bash
# List namespaces
i3x namespaces

# List object types
i3x types

# Query specific type schema
i3x types query work-center-type

# List relationship types
i3x rel-types
```

### 3. Object Exploration & Filtering
```bash
# Tree view of all objects
i3x objects --format tree

# Filter by type ID (server-side GET /v1/objects?typeElementId=...)
i3x objects --type work-center-type

# Filter by wildcard pattern or substring (element ID, name, or type)
i3x objects --filter "*pump*"

# Filter specifically by display name
i3x objects --name "Station A"

# Filter by parent ID or composition hierarchy
i3x objects --parent factory-line1 --composition

# Filter to root instances only
i3x objects --root

# Query specific objects by ID with metadata
i3x objects query pump-station --metadata

# Query related objects
i3x related pump-station
```

### 4. Reading & Writing Real-Time Data
```bash
# Read last known values
i3x read pump-station pump-101

# Write value with quality
i3x write pump-station 42.5 --quality Good

# Batch write multiple values
i3x write pump-station=42.5 pump-101=80.0
```

### 5. Historical Telemetry
```bash
# Query history over last 2 hours
i3x history pump-station --start -2h

# Query with explicit RFC3339 timestamps
i3x history pump-station --start 2026-08-15T00:00:00Z --end 2026-08-15T08:00:00Z
```

### 6. Real-Time Streaming & Subscriptions
```bash
# High-level one-command live stream (auto creates subscription and cleans up on exit)
i3x watch pump-station pump-101

# Manual subscription lifecycle
i3x sub create --name "My Subscription"
i3x sub register <subscriptionId> pump-station pump-101
i3x sub stream <subscriptionId>
i3x sub sync <subscriptionId>
i3x sub delete <subscriptionId>
```
