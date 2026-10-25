# MonsterMQ GraphQL Schemas & Comparison Tool

This directory contains the GraphQL SDL schemas for MonsterMQ Core and Edge Broker, along with tooling to fetch live schemas from running brokers and compare them.

## Files

- `main.gql`: GraphQL SDL schema exported from a running MonsterMQ Core.
- `edge.gql`: GraphQL SDL schema exported from a running MonsterMQ Edge Broker.
- Helper commands in the repository root's `make.py` (Python 3.9+, any OS):
  - `python make.py gql-fetch`: fetch live GraphQL SDL schemas from running brokers.
  - `python make.py gql-compare`: run the schema comparison program.
  - `python make.py build --gql`: compile the native or cross-compiled `compare-schemas` binary.
- `main.go`: Standalone Go tool to parse, diff, and compare GraphQL schemas.

---

## Fetching Schemas from Running Brokers

Use `python make.py gql-fetch` (from the repository root) to update `main.gql` and `edge.gql` from running brokers:

```bash
# Fetch both Core (localhost:4000) and Edge (localhost:4001)
python make.py gql-fetch

# Fetch only MonsterMQ Core
python make.py gql-fetch -m http://localhost:4000/graphql

# Fetch only Edge Broker
python make.py gql-fetch -e http://localhost:4001/graphql

# Fetch both and immediately compare differences
python make.py gql-fetch --compare
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
python make.py gql-compare      # from the repository root
# or, inside gql/
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

Use `python make.py build --gql` (from the repository root) to compile the standalone binary into `gql/bin/`:

```bash
# Build native binary (gql/bin/compare-schemas, .exe on Windows)
python make.py build --gql

# Cross-compile for Linux, macOS, and Windows
python make.py build --gql --all

# Clean build artifacts
python make.py build --gql --clean
```

When built, `python make.py gql-compare` automatically uses `gql/bin/compare-schemas` directly.

---

## Running Tests

```bash
go test -v ./...
```
