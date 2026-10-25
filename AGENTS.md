# Agent Guidelines for MonsterMQ Tools (`monster-mq-tools`)

Welcome! This repository contains tools, utilities, and applications for MonsterMQ Full Broker and MonsterMQ Edge Broker.

---

## Repository Structure

- [`cli/`](cli): The official Go command-line tool `mmq` (MonsterMQ CLI) for MonsterMQ GraphQL interfaces.
- [`i3x/`](i3x): The official Go command-line tool `i3x` (i3X CLI) for the i3X 1.0 REST API specification.
- [`mbp/`](mbp): The official Go command-line & TUI tool `mbp` (Monster Build Pipeline) for inspecting, building, observing, and publishing all MonsterMQ components.
- [`monstermq-cli`](.agents/skills/monstermq-cli/SKILL.md)
- [`monstermq-i3x`](.agents/skills/monstermq-i3x/SKILL.md)
- [`monstermq-pipeline`](.agents/skills/monstermq-pipeline/SKILL.md)
- [`monstermq-hmi-builder`](.agents/skills/monstermq-hmi-builder/SKILL.md)
- [`monstermq-graphql`](.agents/skills/monstermq-graphql/SKILL.md)

---

## 🚨 Mandatory Rule: Maintaining Skill Files & Documentation

Whenever you modify, add, or deprecate any feature, command, flag, or configuration option in `mmq` or any tool in this repository:

1. **Skill File Maintenance**: You **MUST** update the corresponding skill instruction file located at:
   - [`.agents/skills/monstermq-cli/SKILL.md`](.agents/skills/monstermq-cli/SKILL.md)
   
   Ensure that all newly introduced commands, flags, aliases, environment variables, or broker capabilities are accurately documented in `SKILL.md`.

2. **Documentation Synchronization**: Ensure [`cli/README.md`](cli/README.md) and repository [`README.md`](README.md) are updated in tandem with `SKILL.md`.

3. **Verification**: Always run unit tests (`cd cli && go test ./...`) and verify CLI flag parsing before committing changes.

---

## Architecture & Conventions

- **GraphQL Interface**: `mmq` communicates with MonsterMQ via HTTP POST GraphQL requests (`cli/client.go`).
- **Configuration Precedence**: Flags > Environment Variables > `.env` file > Defaults (`http://localhost:4000/graphql`).
- **Broker Target Support**:
  - **Full Broker**: Supports complete historical archives, daily counts, and multi-topic aggregations.
  - **Edge Broker**: Supports localized topic value inspection/publishing, edge device sync, and feature discovery (`features`).
- **Build System**: All tools are built by the cross-platform `make.py` in the repository root (`python make.py build [--cli|--i3x|--gql|--mbp] [--all|--clean]`) into `<tool>/bin/`. Publishing is `python make.py publish`; GraphQL schema helpers are `python make.py gql-fetch` / `gql-compare`. When these commands change, update the skill files and READMEs that document them.

---

## Testing & Verification

- Run unit tests: `cd cli && go test ./...`
- Verify CLI execution:
  - Linux/macOS: `python3 make.py build --cli && cli/bin/mmq --help`
  - Windows: `py make.py build --cli; .\cli\bin\mmq.exe --help`
