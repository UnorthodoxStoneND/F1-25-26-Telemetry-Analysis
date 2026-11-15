"""Check the built ZIP, version consistency and publishable contents."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
EMAIL = re.compile(r"[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}")
PERSONAL_PATH = re.compile(r"[A-Za-z]:[\\/]Users[\\/][^\\/\s]+", re.IGNORECASE)


def privacy(name, raw):
    # Replacement characters preserve byte boundaries, avoiding false matches
    # formed by joining unrelated instructions while decoding an executable.
    for encoding in ("utf-8", "utf-16-le"):
        text = raw.decode(encoding, errors="replace")
        if EMAIL.search(text) or PERSONAL_PATH.search(text):
            raise AssertionError(f"Email address or personal Windows path in {name}")


def privacy_files(archive):
    with zipfile.ZipFile(archive) as z:
        for name in z.namelist():
            privacy(name, z.read(name))
    tracked = subprocess.check_output(["git", "-C", str(ROOT), "ls-files", "-z"])
    for name in tracked.decode().split("\0"):
        if name:
            privacy(name, (ROOT / name).read_bytes())
    print("PASS: source and ZIP contain no email addresses or personal Windows paths")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archive", type=Path, help="Use an isolated check package instead of dist")
    parser.add_argument("--check", choices=("all", "integrity", "privacy"), default="all")
    args = parser.parse_args()
    manifest = json.loads((ROOT / "swofty.json").read_text(encoding="utf-8"))
    version = manifest["version"]
    archive = args.archive or ROOT / "dist" / f"UDP_Telemetry_v{version}.zip"
    if args.check == "privacy":
        privacy_files(archive)
        return
    source = (ROOT / "source/main.go").read_text(encoding="utf-8")
    assert f'const addonVersion = "{version}"' in source
    assert f"## {version} " in (ROOT / "CHANGELOG.md").read_text(encoding="utf-8")
    assert f"UDP_Telemetry_v{version}.zip" in (ROOT / "README.md").read_text(encoding="utf-8")
    assert manifest["compat"]["launcher"] == ">=1.9.55"
    assert manifest["launcher"]["program"]["windows"] == "bin/udp-telemetry.exe"
    with zipfile.ZipFile(archive) as z:
        names = z.namelist()
        assert z.testzip() is None and len(names) == len(set(names))
        expected = {"swofty.json", "README.md", "CHANGELOG.md", "LICENSE", "THIRD_PARTY_NOTICES.md", "bin/udp-telemetry.exe", "source/go.mod"}
        assert expected <= set(names)
        for name in names:
            path = PurePosixPath(name)
            assert not path.is_absolute() and ".." not in path.parts and "\\" not in name
            assert name in expected or (path.parent == PurePosixPath("source") and path.suffix in {".go", ".py"})
            raw = z.read(name)
            assert raw == (ROOT / name).read_bytes(), f"Stale package file: {name}"
        assert json.loads(z.read("swofty.json")) == manifest
    with tempfile.TemporaryDirectory() as temporary:
        subprocess.run([sys.executable, str(ROOT / "source/package.py"), "--output", temporary], check=True)
        repeated = Path(temporary) / archive.name
        assert hashlib.sha256(repeated.read_bytes()).digest() == hashlib.sha256(archive.read_bytes()).digest(), "ZIP is not reproducible"
    print("PASS: versions, complete ZIP, reproducibility, no images/recordings/caches")
    if args.check == "all":
        privacy_files(archive)


if __name__ == "__main__":
    main()
