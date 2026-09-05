# MonsterMQ GraphQL Schemas & Comparison Tool

This directory contains the GraphQL SDL schemas for MonsterMQ Full (Main) Broker and Edge Broker, along with tooling to fetch live schemas from running brokers and compare them.

## Files

- `main.gql`: GraphQL SDL schema exported from a running MonsterMQ Main Broker.
- `edge.gql`: GraphQL SDL schema exported from a running MonsterMQ Edge Broker.
- `fetch-schemas.sh`: Shell script to fetch live GraphQL SDL schemas from running brokers.
- `compare-schemas.sh`: Convenience launcher for the schema comparison program.
- `build.sh`: Build script to compile the native or cross-compiled `compare-schemas` binary.
- `main.go`: Standalone Go tool to parse, diff, and compare GraphQL schemas.

---

## Fetching Schemas from Running Brokers

Use `fetch-schemas.sh` to update `main.gql` and `edge.gql` from running brokers:

```bash
# Fetch both Main (localhost:4000) and Edge (localhost:4001)
./fetch-schemas.sh

# Fetch only Main Broker
./fetch-schemas.sh -m http://localhost:4000/graphql

# Fetch only Edge Broker
./fetch-schemas.sh -e http://localhost:4001/graphql

# Fetch both and immediately compare differences
./fetch-schemas.sh --compare
```

---

## Comparing Schemas

The comparison tool parses both SDL files using `gqlparser` and compares:
- Root operations (`Query`, `Mutation`, `Subscription`)
- Type counts & inventory (exclusive to Main vs exclusive to Edge)
- Structural differences in shared types (missing fields, return type differences, argument mismatches, enum values)

### Basic Usage

```bash
# Compare default schemas (main.gql vs edge.gql)
./compare-schemas.sh
# or
go run .

# Show summary and root operations only
go run . -summary

# Compare only a specific type (e.g. Query, Mutation, MqttClientConfig)
go run . -type Query

# Output as Markdown (useful for documentation or reports)
go run . -format markdown

# Output as JSON (useful for CI or automation)
go run . -format json

# Fail with non-zero exit code if differences exist
go run . -fail-on-diff
```

```bash
go run . /path/to/schema1.gql /path/to/schema2.gql
```

---

## Building the Binary

Use `build.sh` to compile the standalone binary into `bin/`:

```bash
# Build native binary (bin/compare-schemas)
./build.sh

# Cross-compile for Linux, macOS, and Windows
./build.sh --all

# Clean build artifacts
./build.sh --clean
```

When built, `./compare-schemas.sh` automatically uses `bin/compare-schemas` directly.

---

## Running Tests

```bash
go test -v ./...
```
