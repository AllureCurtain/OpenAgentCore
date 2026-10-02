"""install.sh writes .env and starts Compose without host Python."""
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALL = ROOT / "deploy/install.sh"


class InstallScriptTests(unittest.TestCase):
    def test_external_proxy_writes_env_and_stops_when_compose_fails(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            log = root / "docker.log"
            self.write_executable(bin_dir / "docker", textwrap.dedent(f"""\
                #!/bin/sh
                printf '%s\\n' "$*" >> {log}
                if [ "$1" = compose ] && [ "$2" = version ]; then
                  printf 'v2.29.1\\n'
                  exit 0
                fi
                if [ "$1" = compose ] && [ "$2" = cp ]; then
                  printf '#!/bin/sh\\n' > ./oac
                  exit 0
                fi
                if [ "$1" = compose ] && [ "$2" = up ]; then
                  echo 'compose failed' >&2
                  exit 1
                fi
                exit 0
                """))
            self.write_executable(bin_dir / "curl", textwrap.dedent("""\
                #!/bin/sh
                output=""
                while [ $# -gt 0 ]; do
                  if [ "$1" = --output ]; then output="$2"; shift 2; continue; fi
                  shift
                done
                printf 'fixture\\n' > "$output"
                """))
            self.write_executable(bin_dir / "sha256sum", textwrap.dedent("""\
                #!/bin/sh
                exit 0
                """))
            install_dir = root / "oac"
            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["HOME"] = str(root)
            completed = subprocess.run(
                ["bash", str(INSTALL), "--install-dir", str(install_dir), "--external-proxy",
                 "--host", "127.0.0.1", "--web-port", "59991", "--public-url", "https://core.example"],
                env=env, capture_output=True, text=True)
            self.assertNotEqual(completed.returncode, 0, completed.stderr)
            self.assertFalse(install_dir.exists(), "a failed first start must remove the directory")
            recorded = log.read_text()
            self.assertIn("compose pull", recorded)
            self.assertIn("compose up -d --wait", recorded)

    def test_help_and_offline_do_not_need_docker(self):
        help_text = subprocess.run(["bash", str(INSTALL), "--help"], capture_output=True, text=True, check=True)
        self.assertIn("--external-proxy", help_text.stdout)
        offline = subprocess.run(["bash", str(INSTALL), "--offline", "/tmp"], capture_output=True, text=True)
        self.assertNotEqual(offline.returncode, 0)
        self.assertIn("not available", offline.stderr)

    def write_executable(self, path, text):
        path.write_text(text)
        path.chmod(path.stat().st_mode | stat.S_IEXEC)


if __name__ == "__main__":
    unittest.main()
