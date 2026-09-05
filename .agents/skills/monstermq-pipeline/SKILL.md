---
name: monstermq-pipeline
description: Comprehensive operational guide and CLI reference for mbp (MonsterMQ Build Pipeline). Use this skill when inspecting component statuses, compiling, building, observing logs, or publishing MonsterMQ components (main, edge, dashboard, explorer, and tools).
---

# MonsterMQ Build Pipeline (`mbp`) Skill Guide

This skill provides operational instructions and command references for using `mbp` to manage, inspect, build, observe, and publish components across the MonsterMQ ecosystem.

---

## Quick Reference & Binary Location

- **Directory**: [`mbp/`](file:///Users/vogler/Workspace/monster/tools/mbp/)
- **Binary Location**: `mbp/bin/mbp` (built via `cd mbp && ./build.sh`)
- **Default Target Components**:
  - `main`: MonsterMQ Main Broker (`../main`)
  - `edge`: MonsterMQ Edge Broker (`../edge`)
  - `dashboard`: MonsterMQ Dashboard (`../dashboard`)
  - `explorer`: MonsterMQ Explorer (`../explorer`)
  - `tools`: MonsterMQ Tools (`.` or `../tools`)
- **Root Resolution**: Looks for parent directory containing `main` and `edge`, or customizable via `--root <path>` / `MONSTER_ROOT`.

---

## Interactive Terminal UI (TUI)

Launch the interactive dashboard:

```bash
cd mbp && ./bin/mbp
```

### Keybindings in TUI

- `[↑/↓]` or `[j/k]`: Select component
- `[b]`: Open build target selector dialog for selected component
- `[B]`: Batch build all components sequentially
- `[p]`: Open publish confirmation dialog for selected component
- `[c]`: Clean build outputs
- `[g]`: Run background `git fetch` to check for updates on remote
- `[r]`: Refresh statuses
- `[f]` or `[l]`: Toggle full-screen live log viewer
- `[a]`: Toggle log autoscroll
- `[x]` or `[Ctrl+C]`: Cancel active build task
- `[?]`: Show help modal
- `[q]`: Quit

---

## Headless CLI Commands

### Status Inspection

```bash
# Print formatted tabular status
./mbp/bin/mbp status

# Output full JSON model
./mbp/bin/mbp status --json
```

### Building Components

```bash
# Build specific component with default target (all)
./mbp/bin/mbp build main
./mbp/bin/mbp build edge
./mbp/bin/mbp build dashboard
./mbp/bin/mbp build explorer
./mbp/bin/mbp build tools

# Build specific target ID
./mbp/bin/mbp build edge --target build-deb
./mbp/bin/mbp build edge --target build-docker
./mbp/bin/mbp build dashboard --target build-mac
./mbp/bin/mbp build main --target build-setup

# Build all components sequentially
./mbp/bin/mbp build all
```

### Publishing Components

```bash
# Publish with confirmation prompt
./mbp/bin/mbp publish edge

# Publish non-interactively
./mbp/bin/mbp publish edge -y
./mbp/bin/mbp publish dashboard --target publish-mac -y
```

### Cleaning Artifacts

```bash
./mbp/bin/mbp clean edge
./mbp/bin/mbp clean all
```
