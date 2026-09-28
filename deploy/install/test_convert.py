"""Historical layout conversion is refused before any install mutation."""
import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import install


class ConvertTests(unittest.TestCase):
    def test_conversion_flags_refuse_without_touching_legacy_state(self):
        base = Path.home() / ".oac/tests/convert"
        base.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=base) as directory:
            root = Path(directory)
            for name, data in (("installation.json", '{"format":1}'),
                               ("core.key", "retained-private-key"), ("history", "retained-history")):
                (root / name).write_text(data)
            before = {p.name: p.read_bytes() for p in root.iterdir()}
            for extra in ([], ["--yes"], ["--public-url", "https://core.example"]):
                output = io.StringIO()
                with self.subTest(extra=extra), contextlib.redirect_stderr(output), self.assertRaises(SystemExit), \
                        mock.patch.object(install, "check_host", side_effect=AssertionError("host touched")):
                    install.main(["--install-dir", str(root), "--convert", *extra])
                self.assertIn("not supported", output.getvalue())
                self.assertIn("reinstall", output.getvalue())
                self.assertEqual(before, {p.name: p.read_bytes() for p in root.iterdir()})

    def test_legacy_layout_refuses_without_conversion_flag(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "installation.json").write_text(json.dumps({"format": 1}))
            before = (root / "installation.json").read_bytes()
            with self.assertRaisesRegex(install.InstallError, "not supported;.*reinstall"):
                install.check_release(root, {"source_commit": "a" * 40})
            self.assertEqual(before, (root / "installation.json").read_bytes())
            self.assertEqual([p.name for p in root.iterdir()], ["installation.json"])
