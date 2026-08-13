# Agent Guidelines for MonsterMQ Tools (`monster-mq-tools`)

Welcome! This repository contains tools, utilities, and applications for MonsterMQ Full Broker and MonsterMQ Edge Broker.

---

## Repository Structure

- [`cli/`](file:///Users/vogler/Workspace/monster/cli/cli): The official Go command-line tool `mmqcli` for MonsterMQ GraphQL interfaces.
- [`.agents/skills/`](file:///Users/vogler/Workspace/monster/cli/.agents/skills): Agent skills and runbooks for MonsterMQ tools (`mmqcli`, `monstermq-hmi-builder`).

---

## 🚨 Mandatory Rule: Maintaining Skill Files & Documentation

Whenever you modify, add, or deprecate any feature, command, flag, or configuration option in `mmqcli` or any tool in this repository:

1. **Skill File Maintenance**: You **MUST** update the corresponding skill instruction file located at:
   - [`.agents/skills/mmqcli/SKILL.md`](.agents/skills/mmqcli/SKILL.md)
   
   Ensure that all newly introduced commands, flags, aliases, environment variables, or broker capabilities are accurately documented in `SKILL.md`.

2. **Documentation Synchronization**: Ensure [`cli/README.md`](cli/README.md) and repository [`README.md`](README.md) are updated in tandem with `SKILL.md`.

3. **Verification**: Always run unit tests (`cd cli && go test ./...`) and verify CLI flag parsing before committing changes.

---

## Architecture & Conventions

- **GraphQL Interface**: `mmqcli` communicates with MonsterMQ via HTTP POST GraphQL requests (`cli/client.go`).
- **Configuration Precedence**: Flags > Environment Variables > `.env` file > Defaults (`http://localhost:4000/graphql`).
- **Broker Target Support**:
  - **Full Broker**: Supports complete historical archives, daily counts, and multi-topic aggregations.
  - **Edge Broker**: Supports localized topic value inspection/publishing, edge device sync, and feature discovery (`features`).
- **Build System**: Cross-compilation and build management are driven by `cli/build.sh` into `cli/bin/`.

---

## Testing & Verification

- Run unit tests: `cd cli && go test ./...`
- Verify CLI execution: `cd cli && ./build.sh && bin/mmqcli --help`
