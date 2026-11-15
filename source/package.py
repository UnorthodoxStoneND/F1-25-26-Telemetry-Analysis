"""Build deterministic installable addon archives from this source tree."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import zipfile

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--output", type=Path, default=root / "dist")
args = parser.parse_args()
version = json.loads((root / "swofty.json").read_text(encoding="utf-8"))["version"]
match = re.search(r'const addonVersion = "([^"]+)"', (root / "source/main.go").read_text(encoding="utf-8"))
if not match or match.group(1) != version:
    raise SystemExit("swofty.json and source/main.go versions must match")
files = [root / name for name in ("swofty.json", "README.md", "CHANGELOG.md", "LICENSE", "THIRD_PARTY_NOTICES.md", "VALIDATION.md", "docs/USAGE.md", "bin/udp-telemetry.exe")]
files += sorted((root / "source").glob("*.go"))
files += sorted((root / "source").glob("*.py"))
files += [root / "source/go.mod"]
for file in files:
    if not file.is_file():
        raise SystemExit(f"Missing {file.relative_to(root)}; build the Windows helper first")
output = args.output.resolve()
output.mkdir(parents=True, exist_ok=True)
addon = output / f"UDP_Telemetry_v{version}.swoftyaddon"
with zipfile.ZipFile(addon, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
    for file in files:
        info = zipfile.ZipInfo(file.relative_to(root).as_posix(), (1980, 1, 1, 0, 0, 0))
        info.compress_type = zipfile.ZIP_DEFLATED
        info.external_attr = 0o100644 << 16
        archive.writestr(info, file.read_bytes())
with zipfile.ZipFile(addon) as archive:
    assert archive.testzip() is None
    assert {"swofty.json", "LICENSE", "bin/udp-telemetry.exe"} <= set(archive.namelist())
    assert not any(name.endswith(".udp.gz") or name.startswith(("recordings/", ".git/")) for name in archive.namelist())
zip_path = addon.with_suffix(".zip")
shutil.copyfile(addon, zip_path)
lines = []
for file in (addon, zip_path):
    digest = hashlib.sha256(file.read_bytes()).hexdigest()
    line = f"{digest}  {file.name}\n"
    Path(str(file) + ".sha256").write_text(line, encoding="utf-8")
    lines.append(line)
    print(f"{file.name}: {file.stat().st_size:,} bytes; SHA-256 {digest}")
(output / "SHA256SUMS.txt").write_text("".join(lines), encoding="utf-8")
