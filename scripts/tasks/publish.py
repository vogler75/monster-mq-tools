"""`make.py publish`: upload the cross-compiled tool binaries to the GitHub release v<version>.

Replaces publish.sh. Differences (hardening):
- refuses pre-release versions (e.g. 0.2.0-dev);
- requires the tag to exist on origin and point at HEAD; it never creates, pushes or
  force-pushes tags (use `make.py release` / `make.py retag`);
- checks `gh auth status` before building or uploading anything;
- uploads only the cross-compiled release binaries (<tool>-<os>-<arch>), not native
  host builds or leftovers such as mbp.exe~;
- uploads are idempotent: an existing release gets `gh release upload --clobber`, so a
  re-run after a network failure completes the job.
"""

import argparse
import datetime
import subprocess
from pathlib import Path
from typing import List

from mmqlib import ui
from mmqlib.cli import Command
from mmqlib.git import Git, GitError
from mmqlib.proc import CommandError, require, run
from mmqlib.version import Version, VersionError, read_version_file

from . import build as tools_build

REMOTE = "origin"


class PublishError(Exception):
    pass


def parse(argv: List[str], default_tag: str) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        prog="make.py publish",
        description=f"Upload the MonsterMQ Tools release binaries to the GitHub release {default_tag}.")
    p.add_argument("-a", "--all", action="store_true",
                   help="publish all tools' release binaries (the default; kept for compatibility)")
    p.add_argument("-b", "--build", action="store_true",
                   help="cross-compile all tools first (make.py build --all)")
    p.add_argument("-y", "--yes", action="store_true", help="don't ask for confirmation")
    p.add_argument("-t", "--tag", default=default_tag, help=f"release tag (default: {default_tag})")
    # publish.sh also accepted the bare words 'all' and 'help'
    argv = ["--all" if a == "all" else "--help" if a == "help" else a for a in argv]
    return p.parse_args(argv)


def check_version(tag: str) -> Version:
    try:
        version = Version.parse(tag)
    except VersionError as e:
        raise PublishError(f"tag '{tag}': {e}")
    if not tag.startswith("v"):
        raise PublishError(f"release tags are named v<version>, got '{tag}'")
    if version.is_prerelease:
        raise PublishError(f"{tag} is a pre-release version; GitHub releases are only published "
                           "for final versions (run 'python make.py release' first)")
    return version


def gh_path() -> str:
    try:
        return require("gh")
    except CommandError:
        raise PublishError("GitHub CLI 'gh' is not installed (https://cli.github.com/)")


def check_gh_auth(root: Path, gh: str) -> None:
    proc = subprocess.run([gh, "auth", "status"], cwd=root, capture_output=True, text=True,
                          encoding="utf-8", errors="replace")
    if proc.returncode != 0:
        raise PublishError("GitHub CLI is not authenticated; run: gh auth login")


def check_tag(git: Git, tag: str) -> str:
    """The tag must be on origin and point at HEAD. Returns the commit."""
    head = git.head()
    remote = git.remote_tags(REMOTE).get(tag)
    if remote is None:
        raise PublishError(f"tag {tag} does not exist on {REMOTE}; create it with "
                           "'python make.py release' (publish never creates tags)")
    if remote.commit != head:
        raise PublishError(
            f"tag {tag} on {REMOTE} points at {remote.commit[:12]}, but HEAD is {head[:12]}.\n"
            f"Check out the tagged commit, or move the tag with 'python make.py retag' "
            "(publish never force-pushes tags)")
    local = git.local_tags().get(tag)
    if local is not None and local.commit != remote.commit:
        ui.warn(f"local tag {tag} ({local.commit[:12]}) differs from {REMOTE}; "
                f"the release uses the tag on {REMOTE}")
    dirty = [line for line in git.status_lines() if not line.startswith("??")]
    if dirty:
        ui.warn("working tree has uncommitted changes; binaries built now may not match the tag")
    return head


def artifacts(root: Path) -> List[Path]:
    return tools_build.release_artifacts(root)


def report_stale(git: Git, files: List[Path]) -> None:
    ts = int(git.out("log", "-1", "--format=%ct", "HEAD"))
    stale = [f for f in files if f.stat().st_mtime < ts]
    if stale:
        when = datetime.datetime.fromtimestamp(ts).strftime("%Y-%m-%d %H:%M")
        ui.warn(f"{len(stale)} artifact(s) are older than the tagged commit ({when}); "
                "use --build to rebuild them from the tag")


def publish(root: Path, args: argparse.Namespace) -> int:
    tag = args.tag
    ui.header(f"MonsterMQ Tools GitHub Publisher ({tag})")
    check_version(tag)
    gh = gh_path()
    check_gh_auth(root, gh)
    git = Git(root)
    check_tag(git, tag)

    if args.build:
        ui.step("Building tools before publishing...")
        tools_build.build(root, tools_build.TOOLS, cross=True)

    expected = artifacts(root)
    present = [f for f in expected if f.is_file()]
    if not present:
        ui.warn("No release binaries found in the tool bin/ directories.")
        ui.confirm("Build all tools now (make.py build --all)?", args.yes)
        tools_build.build(root, tools_build.TOOLS, cross=True)
        present = [f for f in expected if f.is_file()]
    missing = [f for f in expected if not f.is_file()]
    if missing:
        names = "\n".join(f"  {f.relative_to(root).as_posix()}" for f in missing)
        raise PublishError(f"missing release binaries (run with --build):\n{names}")

    report_stale(git, present)
    ui.info(f"Tools artifacts to upload for {tag}:")
    for f in present:
        ui.info(f"  * {f.relative_to(root).as_posix()} ({tools_build._size(f)})")
    ui.info("")
    ui.confirm(f"Upload these artifacts to GitHub release {tag}?", args.yes)

    files = [str(f) for f in present]
    exists = subprocess.run([gh, "release", "view", tag], cwd=root, capture_output=True,
                            text=True, encoding="utf-8", errors="replace").returncode == 0
    if exists:
        ui.step(f"Uploading artifacts to existing GitHub release {tag} (--clobber)...")
        run([gh, "release", "upload", tag, *files, "--clobber"], cwd=root)
    else:
        ui.step(f"Creating new GitHub release {tag}...")
        notes = root / "releases" / f"{tag}.txt"
        notes_args = ["--notes-file", str(notes)] if notes.is_file() else ["--generate-notes"]
        # --verify-tag: never let gh create the tag; it must already be on origin.
        run([gh, "release", "create", tag, *files, "--verify-tag",
             "--title", f"MonsterMQ Tools {tag}", *notes_args], cwd=root)
    ui.ok(f"MonsterMQ Tools binaries published to GitHub release {tag}")
    return 0


def command(root: Path) -> Command:
    def _run(argv: List[str]) -> int:
        try:
            default_tag = "v" + read_version_file(root / "version.txt")
        except VersionError as e:
            ui.error(str(e))
            return 1
        args = parse(argv, default_tag)
        try:
            return publish(root, args)
        except (PublishError, GitError, CommandError, ui.Abort) as e:
            ui.error(str(e))
            return 1
    return Command("upload release binaries to the GitHub release v<version> (-b builds first, -y)", _run)
