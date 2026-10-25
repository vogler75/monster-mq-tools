"""`make.py build`: native and cross-compiled Go binaries for cli (mmq), i3x, gql and mbp.

Replaces build.sh/build.ps1 and the per-tool cli|i3x|gql|mbp/build.{sh,ps1}.
Output names and locations are unchanged:

  native : <tool>/bin/<binary>[.exe]
  --all  : <tool>/bin/<binary>-<os>-<arch>[.exe]   (plus the native binary)
"""

import argparse
import os
import shutil
from dataclasses import dataclass
from pathlib import Path
from typing import Dict, List, Optional, Sequence, Tuple

from mmqlib import ui
from mmqlib.cli import Command
from mmqlib.proc import require, run
from mmqlib.version import read_version_file

# (GOOS, GOARCH, GOARM, artifact suffix)
Target = Tuple[str, str, Optional[str], str]

LINUX_DARWIN_WINDOWS: List[Target] = [
    ("linux", "amd64", None, "linux-amd64"),
    ("linux", "arm64", None, "linux-arm64"),
    ("linux", "arm", "7", "linux-armv7"),
    ("darwin", "amd64", None, "darwin-amd64"),
    ("darwin", "arm64", None, "darwin-arm64"),
    ("windows", "amd64", None, "windows-amd64"),
]
# mbp/build.sh never built linux/armv7 (only mbp/build.ps1 did); build.sh wins.
NO_ARMV7: List[Target] = [t for t in LINUX_DARWIN_WINDOWS if t[3] != "linux-armv7"]


@dataclass(frozen=True)
class Tool:
    key: str                      # command-line flag / directory name
    title: str                    # shown in the build header
    binary: str                   # output base name in <key>/bin/
    package: str                  # go package to build, relative to <key>/
    default_version: str          # used when no version.txt is found
    own_version_first: bool       # mbp: <key>/version.txt wins over the repo version.txt
    targets: Sequence[Target]


TOOLS: List[Tool] = [
    Tool("cli", "MonsterMQ CLI", "mmq", ".", "0.1.0", False, LINUX_DARWIN_WINDOWS),
    Tool("i3x", "i3X CLI", "i3x", ".", "1.0.0", False, LINUX_DARWIN_WINDOWS),
    Tool("gql", "GraphQL Tools", "compare-schemas", ".", "0.1.0", False, LINUX_DARWIN_WINDOWS),
    Tool("mbp", "Build Pipeline", "mbp", "./cmd/mbp", "0.1.0", True, NO_ARMV7),
]
TOOLS_BY_KEY: Dict[str, Tool] = {t.key: t for t in TOOLS}

GOFLAGS = ["-trimpath"]


def exe_suffix() -> str:
    return ".exe" if os.name == "nt" else ""


def native_name(tool: Tool) -> str:
    return tool.binary + exe_suffix()


def target_name(tool: Tool, target: Target) -> str:
    goos, _, _, suffix = target
    return f"{tool.binary}-{suffix}" + (".exe" if goos == "windows" else "")


def release_artifacts(root: Path, tools: Sequence[Tool] = TOOLS) -> List[Path]:
    """The cross-compiled files `build --all` produces (what `publish` uploads)."""
    return [root / t.key / "bin" / target_name(t, tg) for t in tools for tg in t.targets]


def tool_version(root: Path, tool: Tool) -> str:
    repo_file = root / "version.txt"
    own_file = root / tool.key / "version.txt"
    order = [own_file, repo_file] if tool.own_version_first else [repo_file, own_file]
    for path in order:
        if path.is_file():
            value = read_version_file(path)
            if value:
                return value
    return tool.default_version


def _size(path: Path) -> str:
    n = float(path.stat().st_size)
    for unit in ("B", "K", "M", "G"):
        if n < 1024 or unit == "G":
            return f"{n:.0f}{unit}" if unit == "B" else f"{n:.1f}{unit}"
        n /= 1024
    return ""


def go_build(tool_dir: Path, out: str, package: str, version: str, env: Dict[str, Optional[str]]) -> None:
    ldflags = f"-s -w -X main.Version={version}"
    shown = " ".join(f"{k}={v}" for k, v in env.items() if v is not None)
    ui.detail(f"{shown} go build {' '.join(GOFLAGS)} -ldflags=\"{ldflags}\" -o {out} {package}")
    run(["go", "build", *GOFLAGS, f"-ldflags={ldflags}", "-o", out, package],
        cwd=tool_dir, echo=False, env=env)


def move_aside_running_exe(path: Path) -> None:
    """Windows can't overwrite a running .exe (e.g. mbp.exe while mbp runs this build), but it
    can rename it. Move the old binary to <name>.exe~ (gitignored) so the build can write."""
    if os.name != "nt" or not path.is_file():
        return
    for old in sorted(path.parent.glob(path.stem + "*.exe~")):
        try:
            old.unlink()
        except OSError:
            pass  # still running too; keep it
    try:
        with open(path, "r+b"):
            return  # not in use; go build overwrites it
    except OSError:
        pass
    for n in range(0, 10):
        aside = path.with_name(f"{path.stem}{'.' + str(n) if n else ''}.exe~")
        if aside.exists():
            continue
        try:
            path.rename(aside)
            ui.detail(f"{path.name} is in use; moved it to {aside.name}")
        except OSError:
            pass
        return


def build_tool(root: Path, tool: Tool, cross: bool) -> None:
    tool_dir = root / tool.key
    version = tool_version(root, tool)
    (tool_dir / "bin").mkdir(parents=True, exist_ok=True)

    ui.info(f"Building native {tool.binary} binary (version {version})...")
    out = f"bin/{native_name(tool)}"
    move_aside_running_exe(tool_dir / out)
    go_build(tool_dir, out, tool.package, version, {"CGO_ENABLED": "0"})
    ui.ok(f"Native binary built at: {tool.key}/{out}")

    if cross:
        ui.info(f"Cross-compiling {tool.binary} for all targets (version {version})...")
        for target in tool.targets:
            goos, goarch, goarm, _ = target
            env = {"GOOS": goos, "GOARCH": goarch, "GOARM": goarm, "CGO_ENABLED": "0"}
            go_build(tool_dir, f"bin/{target_name(tool, target)}", tool.package, version, env)
        ui.ok(f"All binaries built in: {tool.key}/bin/")
        for path in sorted((tool_dir / "bin").iterdir()):
            if path.is_file():
                ui.detail(f"{path.name:<40} {_size(path):>8}")


def clean(root: Path, tools: Sequence[Tool]) -> None:
    ui.step(f"Cleaning bin/ of: {', '.join(t.key for t in tools)}")
    for tool in tools:
        shutil.rmtree(root / tool.key / "bin", ignore_errors=True)
    ui.ok("Cleaned")


def build(root: Path, tools: Sequence[Tool], cross: bool) -> None:
    require("go")
    mode = "--all" if cross else "--native"
    ui.header(f"Building MonsterMQ Tools ({mode})")
    for tool in tools:
        ui.step(f"\n>>> [{tool.title}] Building {tool.key}...")
        build_tool(root, tool, cross)
    ui.info("")
    ui.ok("All selected MonsterMQ Tools built successfully!")


def parse(argv: List[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        prog="make.py build",
        description="Build the MonsterMQ Tools Go binaries into <tool>/bin/.",
        epilog="Without a tool flag all tools are built. --all also builds the native binaries.")
    mode = p.add_mutually_exclusive_group()
    mode.add_argument("-a", "--all", action="store_true",
                      help="cross-compile for linux, darwin and windows (plus native)")
    mode.add_argument("-n", "--native", action="store_true",
                      help="build native binaries for this host (default)")
    for tool in TOOLS:
        p.add_argument(f"--{tool.key}", action="store_true",
                       help=f"build {tool.binary} ({tool.key}/bin/) only; combinable")
    p.add_argument("-c", "--clean", action="store_true",
                   help="remove the bin/ directories of the selected tools (default: all) and exit")
    return p.parse_args(argv)


def selected_tools(args: argparse.Namespace) -> List[Tool]:
    chosen = [t for t in TOOLS if getattr(args, t.key)]
    return chosen or list(TOOLS)


def command(root: Path) -> Command:
    def _run(argv: List[str]) -> int:
        args = parse(argv)
        tools = selected_tools(args)
        if args.clean:
            clean(root, tools)
        else:
            build(root, tools, cross=args.all)
        return 0
    return Command("build Go binaries (--all cross-compiles; --cli/--i3x/--gql/--mbp; --clean)", _run)
