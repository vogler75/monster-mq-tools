# Agent Guidelines for MonsterMQ CLI (`mmqcli`)

Welcome! This repository contains `mmqcli`, the official Go command-line tool for MonsterMQ Full Broker and MonsterMQ Edge Broker GraphQL interfaces.

---

## 🚨 Mandatory Rule: Maintaining the Skill File

Whenever you modify, add, or deprecate any feature, command, flag, or configuration option in `mmqcli`:

1. **Skill File Maintenance**: You **MUST** update the skill instruction file located at:
   - [`.agents/skills/mmqcli/SKILL.md`](.agents/skills/mmqcli/SKILL.md)
   
   Ensure that all newly introduced commands, flags, aliases, environment variables, or broker capabilities are accurately documented in `SKILL.md`.

2. **Documentation Synchronization**: Ensure `README.md` is updated in tandem with `SKILL.md`.

3. **Verification**: Always run `go test ./...` and verify CLI flag parsing before committing changes.

---

## Architecture & Conventions

- **GraphQL Interface**: `mmqcli` communicates with MonsterMQ via HTTP POST GraphQL requests (`client.go`).
- **Configuration Precedence**: Flags > Environment Variables > `.env` file > Defaults (`http://localhost:4000/graphql`).
- **Broker Target Support**:
  - **Full Broker**: Supports complete historical archives, daily counts, and multi-topic aggregations.
  - **Edge Broker**: Supports localized topic value inspection/publishing, edge device sync, and feature discovery (`features`).
- **Build System**: Cross-compilation and build management are driven by `build.sh` into `bin/`.

---

## Testing & Verification

- Run unit tests: `go test ./...`
- Verify CLI execution: `./build.sh && bin/mmqcli --help`
