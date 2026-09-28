"""Current lifecycle commands never resume historical rename/conversion journals."""
import json
from pathlib import Path
import tempfile
import unittest

import oac_cli


class RenameTests(unittest.TestCase):
    def test_old_format_and_all_conversion_journals_refuse_before_lock(self):
        base = Path.home() / ".oac/tests/rename"
        base.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=base) as directory:
            root = Path(directory)
            for record in ({"format": 1}, *({"format": 2, key: {"finished": finished}}
                         for key in ("converted_from", "renamed_from") for finished in (False, True))):
                path = root / "state.json"
                path.write_text(json.dumps(record))
                path.chmod(0o600)
                before = path.read_bytes()
                with self.subTest(record=record), self.assertRaisesRegex(oac_cli.OacError, "not supported;.*reinstall"):
                    with oac_cli.locked(root):
                        self.fail("Unsupported installation acquired the lock")
                self.assertEqual(before, path.read_bytes())
                self.assertFalse((root / ".oac.lock").exists())
