"""`make.py gql-fetch` and `make.py gql-compare`: GraphQL schema helpers in gql/.

Replace gql/fetch-schemas.sh and gql/compare-schemas.sh.
"""

import argparse
import os
import subprocess
import tempfile
from pathlib import Path
from typing import List

from mmqlib import ui
from mmqlib.cli import Command
from mmqlib.proc import CommandError, require, run, which

from .build import _size, exe_suffix

DEFAULT_MAIN_URL = "http://localhost:4000/graphql"
DEFAULT_EDGE_URL = "http://localhost:4001/graphql"

FETCH_EPILOG = f"""\
URLs:
  fetching both (default): [MAIN_URL] [EDGE_URL]
  with -m or -e only     : [URL] for that broker
  defaults: {DEFAULT_MAIN_URL} (core), {DEFAULT_EDGE_URL} (edge)
  The environment variables MAIN_URL / EDGE_URL override the defaults and the arguments.

examples:
  python make.py gql-fetch                                  # both brokers, default URLs
  python make.py gql-fetch -m                               # MonsterMQ Core only
  python make.py gql-fetch -e http://host:4001/graphql      # Edge from a custom URL
  python make.py gql-fetch http://m:4000/graphql http://e:4001/graphql
  python make.py gql-fetch --compare                        # fetch, then compare
"""

COMPARE_HELP = """\
usage: python make.py gql-compare [tool options] [MAIN.gql EDGE.gql]

Compare two GraphQL SDL schemas (default: gql/main.gql vs gql/edge.gql).
Uses gql/bin/compare-schemas if built (python make.py build --gql), else 'go run .'.
All arguments are passed to the tool; relative schema paths are taken from the
current directory.

tool options:
  -summary          show a high-level comparison summary only
  -type NAME        compare only one type (e.g. Query, Device)
  -format FORMAT    text (default), markdown or json
  -fail-on-diff     exit with code 1 if differences are found
"""


def fetch_schema(name: str, url: str, target: Path, npx: str) -> bool:
    """Fetch via a temp file so an existing valid schema is never overwritten on failure."""
    fd, tmp = tempfile.mkstemp(dir=str(target.parent), prefix=f".{target.name}.", suffix=".tmp")
    try:
        with os.fdopen(fd, "wb") as out:
            proc = subprocess.run([npx, "-y", "get-graphql-schema", url], stdout=out,
                                  stderr=subprocess.PIPE)
        if proc.returncode == 0 and os.path.getsize(tmp) > 0:
            os.replace(tmp, target)
            ui.ok(f"{name} schema saved to {target.parent.name}/{target.name} ({_size(target)})")
            return True
        ui.error(f"Failed to fetch {name} schema from {url}")
        err = proc.stderr.decode("utf-8", errors="replace").strip()
        for line in (err.splitlines() or ["(Server offline, unreachable, or returned an empty schema)"]):
            ui.info(f"    {line}")
        return False
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def compare(gql_dir: Path, tool_args: List[str]) -> int:
    binary = gql_dir / "bin" / ("compare-schemas" + exe_suffix())
    cmd = [str(binary)] if binary.is_file() else ["go", "run", "."]
    return run([*cmd, *tool_args], cwd=gql_dir, check=False)


def fetch_command(root: Path) -> Command:
    gql_dir = root / "gql"

    def _run(argv: List[str]) -> int:
        p = argparse.ArgumentParser(
            prog="make.py gql-fetch", formatter_class=argparse.RawDescriptionHelpFormatter,
            description="Fetch GraphQL SDL schemas from running MonsterMQ Core & Edge brokers "
                        "into gql/main.gql and gql/edge.gql (uses 'npx get-graphql-schema').",
            epilog=FETCH_EPILOG)
        p.add_argument("-m", "--main", "--main-only", action="store_true", help="fetch only the MonsterMQ Core schema")
        p.add_argument("-e", "--edge", "--edge-only", action="store_true", help="fetch only the Edge Broker schema")
        p.add_argument("-a", "--all", action="store_true", help="fetch both schemas (default)")
        p.add_argument("-c", "--compare", action="store_true", help="compare the schemas after fetching")
        p.add_argument("urls", nargs="*", metavar="URL")
        args = p.parse_args(argv)

        fetch_main = args.main or args.all
        fetch_edge = args.edge or args.all
        if not fetch_main and not fetch_edge:
            fetch_main = fetch_edge = True
        selected = [s for s, on in (("main", fetch_main), ("edge", fetch_edge)) if on]
        if len(args.urls) > len(selected):
            p.error(f"too many URLs for {' and '.join(selected)}: {' '.join(args.urls)}")
        given = dict(zip(selected, args.urls))  # positional URLs in the order main, edge
        jobs = []
        if fetch_main:
            url = os.environ.get("MAIN_URL") or given.get("main") or DEFAULT_MAIN_URL
            jobs.append(("MonsterMQ Core", url, gql_dir / "main.gql"))
        if fetch_edge:
            url = os.environ.get("EDGE_URL") or given.get("edge") or DEFAULT_EDGE_URL
            jobs.append(("Edge Broker", url, gql_dir / "edge.gql"))

        try:
            npx = require("npx")
        except CommandError:
            ui.error("'npx' not found on PATH (install Node.js)")
            return 1
        ui.info("Fetching GraphQL schemas...")
        for name, url, _ in jobs:
            ui.info(f"  {name}: {url}")
        ui.info("")
        results = [fetch_schema(name, url, target, npx) for name, url, target in jobs]
        ui.info("")
        if not all(results):
            ui.error("Completed with errors (check if target brokers are running).")
            return 1
        ui.ok("Done!")
        if args.compare:
            ui.step("\nRunning schema comparison...")
            return compare(gql_dir, [str(gql_dir / "main.gql"), str(gql_dir / "edge.gql")])
        return 0

    return Command("fetch GraphQL schemas from running brokers into gql/*.gql (-m, -e, -c)", _run)


def compare_command(root: Path) -> Command:
    gql_dir = root / "gql"

    def _run(argv: List[str]) -> int:
        if any(a in ("-h", "--help", "help") for a in argv):
            print(COMPARE_HELP)
            return 0
        if not which("go") and not (gql_dir / "bin" / ("compare-schemas" + exe_suffix())).is_file():
            ui.error("'go' not found on PATH and gql/bin/compare-schemas is not built")
            return 1
        # The tool runs in gql/; make schema paths given relative to the caller absolute.
        args = [str(Path(a).resolve()) if not a.startswith("-") and Path(a).is_file() else a
                for a in argv]
        return compare(gql_dir, args)

    return Command("compare GraphQL schemas (gql/main.gql vs gql/edge.gql); args go to the tool", _run)
