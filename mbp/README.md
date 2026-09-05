# mbp (MonsterMQ Build Pipeline)

`mbp` is the terminal build pipeline, status dashboard, and release orchestrator for the MonsterMQ ecosystem (`main`, `edge`, `dashboard`, `explorer`, and `tools`).

It provides both an interactive **Terminal User Interface (TUI)** built with Charmbracelet Bubble Tea & Lipgloss and a scriptable **Headless CLI** for CI/CD and automation.

---

## Features

- **Automated Ecosystem Discovery**: Automatically locates and inspects all MonsterMQ repositories in the parent workspace:
  - `main` (Main Broker - Java bundle, setup executables, docker)
  - `edge` (Edge Broker - Go native binary, multi-arch Debian packages, docker)
  - `dashboard` (Dashboard - Vite bundle, macOS DMG, Windows NSIS setup)
  - `explorer` (Explorer - Electron desktop packages, macOS DMG, Windows setup, docker)
  - `tools` (Tools - `mmq`, `i3x`, and schema utilities)
- **Real-Time Git Inspection**: Checks current branch, short commit hash, uncommitted/dirty changes, and upstream tracking status (ahead/behind counts indicating if new versions are available on git).
- **Compilation & Freshness Detection**: Discovers compiled artifacts on disk and compares modification times against the latest source commits to show whether components are `Built`, `Outdated`, or `Missing`.
- **Target Selection & Single/Batch Builds**: Build individual targets (e.g. Docker only, Debian only, macOS only) or batch build all components in sequence (`[B]`).
- **Live Output Streaming & Observation**: View real-time `stdout` and `stderr` logs, execution timer, autoscroll toggle, and fullscreen log zoom (`[f]`).
- **Process Cancellation**: Safely terminate long-running or stuck builds along with their entire process groups (`[x]` or `Ctrl+C`).
- **Release Publishing**: Interactive target selection and safeguards to execute each component's `publish.sh` workflow.

---

## Installation & Build

Compile the native binary with the build script:

```bash
cd mbp
./build.sh
```

This compiles the binary into `mbp/bin/mbp`.

To cross-compile for Linux, macOS (Intel & Apple Silicon), and Windows:

```bash
./build.sh --all
```

---

## Interactive Terminal UI (TUI)

Launch the interactive dashboard:

```bash
./mbp/bin/mbp
```

### Keyboard Shortcuts

| Shortcut | Description |
|---|---|
| `↑` / `↓` or `k` / `j` | Move selection between components |
| `b` | Open build target selector dialog for selected component |
| `B` | Start batch **Build All** sequentially |
| `p` | Open publish confirmation dialog for selected component |
| `c` | Clean build outputs for selected component |
| `g` | Run `git fetch` in background & refresh remote statuses |
| `r` | Refresh component & artifact statuses immediately |
| `f` or `l` | Toggle full-screen live log viewer |
| `a` | Toggle log autoscroll on / off |
| `x` / `Ctrl+C` | Cancel / terminate active build process |
| `?` | Toggle keyboard shortcuts help dialog |
| `q` | Quit MBP |

---

## Headless CLI Usage

`mbp` can also be run in headless/scripting mode:

### Inspect Component Statuses

```bash
# Formatted table
./bin/mbp status

# Machine-readable JSON output
./bin/mbp status --json
```

### Build Components

```bash
# Build specific component using default target (all)
./bin/mbp build edge

# Build specific target (e.g. docker only, deb only)
./bin/mbp build edge --target build-deb
./bin/mbp build main --target build-docker
./bin/mbp build dashboard --target build-mac

# Build all components sequentially
./bin/mbp build all
```

### Publish Components

```bash
# Interactive confirmation prompt
./bin/mbp publish edge

# Non-interactive with auto-confirm
./bin/mbp publish edge --target publish-all -y
```

### Clean Artifacts

```bash
# Clean specific component
./bin/mbp clean dashboard

# Clean all components
./bin/mbp clean all
```

---

## Configuration & Environment Variables

- `MONSTER_ROOT`: Path to the directory containing MonsterMQ components (defaults to parent directory `..`).
- `--root <path>`: CLI flag overriding the repository root.
