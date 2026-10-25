#!/usr/bin/env python3
"""MonsterMQ Tools build, release and publish tool - run 'python make.py help'."""

import sys
from pathlib import Path

sys.dont_write_bytecode = True  # no __pycache__ in the tree; a release needs a clean working tree
ROOT = Path(__file__).resolve().parent
sys.path.insert(0, str(ROOT / "scripts"))

from mmqlib.cli import run_cli  # noqa: E402
from mmqlib.release import ReleaseConfig, release_commands  # noqa: E402
from tasks import build, gql, publish  # noqa: E402

RELEASE = ReleaseConfig(
    name="MonsterMQ Tools",
    next_steps=[
        "Build artifacts  : python make.py build --all",
        "Publish assets   : python make.py publish",
        "Next dev version : python make.py dev",
    ],
)

COMMANDS = {
    "build": build.command(ROOT),
    "publish": publish.command(ROOT),
    "gql-fetch": gql.fetch_command(ROOT),
    "gql-compare": gql.compare_command(ROOT),
    **release_commands(RELEASE, ROOT),
}

if __name__ == "__main__":
    sys.exit(run_cli("make.py", "MonsterMQ Tools", COMMANDS))
