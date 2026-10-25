"""Tests for the tools make.py tasks (build, publish) with external commands mocked.

Run from the repo root:  python -m unittest discover -s scripts/tests -v
"""

import contextlib
import io
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mmqlib.git import TagRef  # noqa: E402
from tasks import build, publish  # noqa: E402

HEAD = "a" * 40


def quiet():
    return contextlib.redirect_stdout(io.StringIO())


class FakeGit:
    def __init__(self, remote_commit=HEAD, local_commit=HEAD, status=()):
        self.remote_commit = remote_commit
        self.local_commit = local_commit
        self.status = list(status)

    def head(self):
        return HEAD

    def remote_tags(self, remote):
        return {} if self.remote_commit is None else {"v1.2.3": TagRef(self.remote_commit, self.remote_commit)}

    def local_tags(self):
        return {} if self.local_commit is None else {"v1.2.3": TagRef(self.local_commit, self.local_commit)}

    def status_lines(self):
        return self.status

    def out(self, *args):
        return "0"  # commit timestamp: every artifact is newer


class BuildTest(unittest.TestCase):
    def test_artifact_names_match_old_scripts(self):
        names = {p.relative_to(Path("r")).as_posix() for p in build.release_artifacts(Path("r"))}
        self.assertIn("cli/bin/mmq-linux-armv7", names)
        self.assertIn("cli/bin/mmq-windows-amd64.exe", names)
        self.assertIn("i3x/bin/i3x-darwin-arm64", names)
        self.assertIn("gql/bin/compare-schemas-linux-amd64", names)
        self.assertIn("mbp/bin/mbp-windows-amd64.exe", names)
        self.assertNotIn("mbp/bin/mbp-linux-armv7", names)  # mbp/build.sh never built it
        self.assertEqual(len(names), 6 + 6 + 6 + 5)

    def test_version_precedence(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            (root / "mbp").mkdir()
            (root / "cli").mkdir()
            self.assertEqual(build.tool_version(root, build.TOOLS_BY_KEY["i3x"]), "1.0.0")
            (root / "version.txt").write_text("2.0.0\n")
            (root / "mbp" / "version.txt").write_text("0.3.0\r\n")
            (root / "cli" / "version.txt").write_text("9.9.9\n")
            self.assertEqual(build.tool_version(root, build.TOOLS_BY_KEY["mbp"]), "0.3.0")
            self.assertEqual(build.tool_version(root, build.TOOLS_BY_KEY["cli"]), "2.0.0")

    def test_cross_build_env(self):
        calls = []
        with tempfile.TemporaryDirectory() as d, quiet(), \
                mock.patch.object(build, "run", lambda cmd, **kw: calls.append((cmd, kw))):
            build.build_tool(Path(d), build.TOOLS_BY_KEY["cli"], cross=True)
        self.assertEqual(len(calls), 7)  # native + 6 targets
        native_cmd, native_kw = calls[0]
        self.assertEqual(native_kw["env"], {"CGO_ENABLED": "0"})
        self.assertIn("-ldflags=-s -w -X main.Version=0.1.0", native_cmd)
        arm = [kw["env"] for cmd, kw in calls if "bin/mmq-linux-armv7" in cmd][0]
        self.assertEqual(arm, {"GOOS": "linux", "GOARCH": "arm", "GOARM": "7", "CGO_ENABLED": "0"})
        amd = [kw["env"] for cmd, kw in calls if "bin/mmq-linux-amd64" in cmd][0]
        self.assertIsNone(amd["GOARM"])  # removes an inherited GOARM


class PublishTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        (self.root / "version.txt").write_text("1.2.3\n")
        for f in build.release_artifacts(self.root):
            f.parent.mkdir(parents=True, exist_ok=True)
            f.write_bytes(b"x")
        (self.root / "mbp" / "bin" / "mbp.exe~").write_bytes(b"leftover")

    def tearDown(self):
        self.tmp.cleanup()

    def publish(self, argv, git=None, release_exists=True):
        ran = []

        def fake_subprocess_run(cmd, **kw):
            ok = cmd[1:3] != ["release", "view"] or release_exists
            return subprocess.CompletedProcess(cmd, 0 if ok else 1, "", "")

        with quiet(), \
                mock.patch.object(publish, "Git", lambda root: git or FakeGit()), \
                mock.patch.object(publish, "require", lambda name: "gh"), \
                mock.patch.object(publish.subprocess, "run", fake_subprocess_run), \
                mock.patch.object(publish, "run", lambda cmd, **kw: ran.append(cmd)):
            code = publish.command(self.root).run(argv)
        return code, ran

    def test_existing_release_uploads_with_clobber(self):
        code, ran = self.publish(["-y"])
        self.assertEqual(code, 0)
        self.assertEqual(len(ran), 1)
        cmd = ran[0]
        self.assertEqual(cmd[:4], ["gh", "release", "upload", "v1.2.3"])
        self.assertIn("--clobber", cmd)
        self.assertEqual(len([c for c in cmd if c.startswith(str(self.root))]), 23)
        self.assertFalse(any(c.endswith("mbp.exe~") for c in cmd))

    def test_new_release_verifies_tag_and_uses_notes(self):
        (self.root / "releases").mkdir()
        (self.root / "releases" / "v1.2.3.txt").write_text("notes")
        code, ran = self.publish(["-y"], release_exists=False)
        self.assertEqual(code, 0)
        cmd = ran[0]
        self.assertEqual(cmd[:4], ["gh", "release", "create", "v1.2.3"])
        self.assertIn("--verify-tag", cmd)
        self.assertIn("--notes-file", cmd)

    def test_refuses_prerelease(self):
        (self.root / "version.txt").write_text("1.2.4-dev\n")
        code, ran = self.publish(["-y"])
        self.assertEqual((code, ran), (1, []))

    def test_refuses_tag_not_at_head(self):
        code, ran = self.publish(["-y"], git=FakeGit(remote_commit="b" * 40))
        self.assertEqual((code, ran), (1, []))

    def test_refuses_tag_missing_on_origin(self):
        code, ran = self.publish(["-y"], git=FakeGit(remote_commit=None))
        self.assertEqual((code, ran), (1, []))

    def test_refuses_incomplete_artifacts(self):
        (self.root / "cli" / "bin" / "mmq-linux-arm64").unlink()
        code, ran = self.publish(["-y"])
        self.assertEqual((code, ran), (1, []))

    def test_needs_confirmation_without_tty(self):
        with mock.patch.object(sys.stdin, "isatty", lambda: False):
            code, ran = self.publish([])
        self.assertEqual((code, ran), (1, []))


if __name__ == "__main__":
    unittest.main()
