"""Create the ZIP-only live add-on distribution."""
import argparse
import json
from pathlib import Path, PurePosixPath
import re
import zipfile

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--output', type=Path, default=root/'dist')
args = parser.parse_args()
manifest = json.loads((root/'swofty.json').read_text(encoding='utf-8'))
version = manifest['version']
# GitHub and the Addon Browser receive this same complete package.
assert manifest.get('description', '').strip(), 'Set a browser description'
assert manifest.get('category') == 'utility', 'Set the utility browser category'
assert manifest.get('repository') == 'https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis', 'Set the public source repository'
assert manifest.get('compat', {}).get('platforms') == ['steam'], 'Declare the tested Steam platform'
icon = PurePosixPath(manifest.get('icon', ''))
assert str(icon) != '.' and not icon.is_absolute() and '..' not in icon.parts and '\\' not in str(icon) and ':' not in str(icon), 'Icon must be an add-on-relative path'
assert icon.suffix.lower() in {'.png', '.jpg', '.webp'}, 'Addon Browser icons must be PNG, JPG or WebP'
icon_file = root.joinpath(*icon.parts)
assert icon_file.resolve().is_relative_to(root.resolve()) and icon_file.is_file(), 'Browser icon is missing or outside the add-on'
assert 0 < icon_file.stat().st_size <= 2_000_000, 'Browser icon exceeds 2 MB'

assert re.search(r'const addonVersion = "'+re.escape(version)+r'"', (root/'source/main.go').read_text(encoding='utf-8'))
files = [root/name for name in ('swofty.json','README.md','CHANGELOG.md','LICENSE','THIRD_PARTY_NOTICES.md','bin/telemetry-live.exe','bin/telemetry-live.dll','source/go.mod')]
for folder, extensions in (('source',{'.go','.py'}),('native',{'.h','.cpp','.py','.txt'})):
    files += sorted(p for p in (root/folder).iterdir() if p.suffix in extensions and p.is_file())
files.append(icon_file)
for file in files:
    if not file.is_file(): raise SystemExit(f'Missing {file.relative_to(root)}; build the helper and native component first')
args.output.mkdir(parents=True,exist_ok=True)
addon = args.output/f'F1_Telemetry_v{version}.zip'
with zipfile.ZipFile(addon,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as archive:
    for file in sorted(files):
        info = zipfile.ZipInfo(file.relative_to(root).as_posix(),(1980,1,1,0,0,0))
        info.compress_type=zipfile.ZIP_DEFLATED;info.external_attr=0o100644<<16
        archive.writestr(info,file.read_bytes())
with zipfile.ZipFile(addon) as archive: assert archive.testzip() is None
print(f'{addon.name}: {addon.stat().st_size:,} bytes')
