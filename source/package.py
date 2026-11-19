"""Package the live add-on; rotate local test builds without changing Releases."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]


def is_test(version):
    return re.search(r'^\d+\.\d+\.\d+-test(?:[.+-]|$)', version) is not None


def default_output(root, version):
    return root.parent/'Testing' if is_test(version) else root/'dist'


def checksum(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def checksum_text(digest, filename):
    return f'{digest}  {filename}\n'


def archive_plan(output, current_name, current_hash):
    """Preflight every move before changing the existing test packages."""
    plans = []
    for previous in sorted(output.glob('F1_Telemetry_v*.zip')):
        version = previous.name[len('F1_Telemetry_v'):-len('.zip')]
        if not is_test(version):
            continue
        if previous.resolve().parent != output:
            raise ValueError(f'Test ZIP resolves outside Testing: {previous.name}')
        digest = checksum(previous)
        if previous.name == current_name and digest == current_hash:
            continue
        sidecar = previous.with_name(previous.name+'.sha256')
        if sidecar.exists() or sidecar.is_symlink():
            if sidecar.resolve().parent != output:
                raise ValueError(f'Checksum resolves outside Testing: {sidecar.name}')
            if sidecar.read_text(encoding='ascii').strip() != checksum_text(digest, previous.name).strip():
                raise ValueError(f'Existing checksum does not match {previous.name}')
        destination = output/'Archive'
        number = 2
        while any(path.exists() or path.is_symlink() for path in (destination/previous.name, destination/sidecar.name)):
            destination = output/'Archive'/previous.stem/f'rebuild-{number}'
            number += 1
        if not destination.resolve().is_relative_to(output) or destination.resolve() == output:
            raise ValueError('Archive resolves outside Testing')
        plans.append((previous, sidecar, destination, digest))
    return plans


def package(root=ROOT, output=None):
    root = root.resolve()
    manifest = json.loads((root/'swofty.json').read_text(encoding='utf-8'))
    version = manifest['version']
    if not re.fullmatch(r'\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?', version):
        raise ValueError('Invalid package version')
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
    files = [root/name for name in ('swofty.json','README.md','CHANGELOG.md','LICENSE','THIRD_PARTY_NOTICES.md','bin/telemetry-live.exe','bin/telemetry-live.dll','bin/telemetry-live-linux','source/go.mod')]
    for folder, extensions in (('source',{'.go','.py'}),('native',{'.h','.hpp','.cpp','.py','.txt'})):
        files += sorted(p for p in (root/folder).iterdir() if p.suffix in extensions and p.is_file())
    files.append(icon_file)
    for file in files:
        if not file.is_file(): raise SystemExit(f'Missing {file.relative_to(root)}; build the helper and native component first')
    output = (output if output is not None else default_output(root, version)).resolve()
    releases = (root.parent/'Releases').resolve()
    if is_test(version) and output.is_relative_to(releases):
        raise ValueError('Test packaging must not write into Releases')
    managed_tests = is_test(version) and output == (root.parent/'Testing').resolve()
    output.mkdir(parents=True, exist_ok=True)
    addon = output/f'F1_Telemetry_v{version}.zip'
    sidecar = addon.with_name(addon.name+'.sha256')
    staged = []
    try:
        for _ in range(2):
            with tempfile.NamedTemporaryFile(dir=output, prefix='.telemetry-', suffix='.tmp', delete=False) as temporary:
                staged.append(Path(temporary.name))
        with zipfile.ZipFile(staged[0], 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            for file in sorted(files):
                info = zipfile.ZipInfo(file.relative_to(root).as_posix(), (1980, 1, 1, 0, 0, 0))
                info.compress_type = zipfile.ZIP_DEFLATED
                info.external_attr = (0o100755 if file.relative_to(root).as_posix()=='bin/telemetry-live-linux' else 0o100644) << 16
                archive.writestr(info, file.read_bytes())
        with zipfile.ZipFile(staged[0]) as archive:
            assert archive.testzip() is None
        digest = checksum(staged[0])
        staged[1].write_text(checksum_text(digest, addon.name), encoding='ascii')
        plans = archive_plan(output, addon.name, digest) if managed_tests else []
        for previous, old_checksum, destination, old_digest in plans:
            destination.mkdir(parents=True, exist_ok=True)
            previous.rename(destination/previous.name)
            if old_checksum.exists():
                old_checksum.rename(destination/old_checksum.name)
            else:
                (destination/old_checksum.name).write_text(checksum_text(old_digest, previous.name), encoding='ascii')
            print(f'Archived {previous.name} -> {destination}')
        staged[0].replace(addon)
        staged[1].replace(sidecar)
    finally:
        for temporary in staged:
            temporary.unlink(missing_ok=True)
    print(f'{addon}: {addon.stat().st_size:,} bytes; SHA-256 in {sidecar.name}')
    return addon


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, help='Isolated output override (e.g. dist for CI); rotates tests only when targeting Testing')
    args = parser.parse_args()
    package(output=args.output)


if __name__ == '__main__':
    main()
