"""Exercise package routing, checksums and archive preservation in isolated workspaces."""
import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('telemetry_package', ROOT/'source/package.py')
packager = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packager)


class PackagingTests(unittest.TestCase):
    def setUp(self):
        scratch = ROOT/'.verify'
        scratch.mkdir(exist_ok=True)
        self.temporary = tempfile.TemporaryDirectory(prefix='packaging-', dir=scratch)
        self.addCleanup(self.temporary.cleanup)
        self.workspace = Path(self.temporary.name)
        self.source = self.workspace/'Source'
        for folder in ('source', 'native', 'bin'):
            (self.source/folder).mkdir(parents=True, exist_ok=True)
        for name in ('README.md', 'CHANGELOG.md', 'LICENSE', 'THIRD_PARTY_NOTICES.md', 'source/go.mod', 'native/SDK-LICENSE.txt', 'bin/telemetry-live.exe', 'bin/telemetry-live.dll', 'bin/telemetry-live-linux'):
            (self.source/name).write_bytes(b'fixture')
        (self.source/'icon.png').write_bytes(b'\x89PNG\r\n\x1a\nfixture')
        shutil.copy2(ROOT/'source/package.py', self.source/'source/package.py')
        self.testing = self.workspace/'Testing'
        self.testing.mkdir()
        self.releases = self.workspace/'Releases'
        self.releases.mkdir()
        (self.releases/'keep.zip').write_bytes(b'release must remain unchanged')
        self.set_version('0.5.9-test.2')

    def set_version(self, version):
        manifest = dict(version=version, description='Live telemetry', category='utility',
                        repository='https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis',
                        compat=dict(platforms=['steam']), icon='icon.png')
        (self.source/'swofty.json').write_text(json.dumps(manifest), encoding='utf-8')
        (self.source/'source/main.go').write_text(f'const addonVersion = "{version}"', encoding='utf-8')

    def build(self, output=None):
        with contextlib.redirect_stdout(io.StringIO()):
            result = packager.package(self.source, output)
        self.assertEqual(list(self.releases.iterdir()), [self.releases/'keep.zip'])
        self.assertEqual((self.releases/'keep.zip').read_bytes(), b'release must remain unchanged')
        return result

    def previous(self, version='0.5.9-test.1', data=b'previous package', sidecar=True, folder=None):
        folder = folder or self.testing
        folder.mkdir(parents=True, exist_ok=True)
        path = folder/f'F1_Telemetry_v{version}.zip'
        path.write_bytes(data)
        if sidecar:
            path.with_name(path.name+'.sha256').write_text(hashlib.sha256(data).hexdigest()+'  '+path.name+'\n', encoding='ascii')
        return path

    def assert_checksum(self, archive):
        expected = hashlib.sha256(archive.read_bytes()).hexdigest()+'  '+archive.name
        self.assertEqual(archive.with_name(archive.name+'.sha256').read_text(encoding='ascii').strip(), expected)

    def test_default_routes_and_archives_pair(self):
        previous = self.previous()
        stable = self.testing/'F1_Telemetry_v0.5.8.zip'
        stable.write_bytes(b'not a test')
        archive = self.build()
        self.assertEqual(archive.parent, self.testing)
        self.assertFalse(previous.exists())
        archived = self.testing/'Archive'/previous.name
        self.assertEqual(archived.read_bytes(), b'previous package')
        self.assert_checksum(archived)
        self.assert_checksum(archive)
        self.assertEqual(stable.read_bytes(), b'not a test')

    def test_identical_rebuild_does_not_archive_current(self):
        archive = self.build()
        digest = archive.read_bytes()
        self.build()
        self.assertEqual(archive.read_bytes(), digest)
        self.assertFalse((self.testing/'Archive').exists())
        self.assert_checksum(archive)

    def test_changed_same_version_preserves_previous(self):
        archive = self.build()
        previous = archive.read_bytes()
        (self.source/'README.md').write_text('changed package contents')
        self.build()
        archived = self.testing/'Archive'/archive.name
        self.assertEqual(archived.read_bytes(), previous)
        self.assertNotEqual(archive.read_bytes(), previous)
        self.assert_checksum(archived)
        self.assert_checksum(archive)

    def test_archive_collision_keeps_both_packages(self):
        previous = self.previous()
        existing = self.previous(data=b'earlier archived package', folder=self.testing/'Archive')
        self.build()
        self.assertEqual(existing.read_bytes(), b'earlier archived package')
        archived = self.testing/'Archive'/previous.stem/'rebuild-2'/previous.name
        self.assertEqual(archived.read_bytes(), b'previous package')
        self.assert_checksum(existing)
        self.assert_checksum(archived)

    def test_missing_previous_checksum_is_generated(self):
        previous = self.previous(sidecar=False)
        self.build()
        self.assert_checksum(self.testing/'Archive'/previous.name)

    def test_invalid_existing_checksum_keeps_packages_in_place(self):
        previous = self.previous()
        previous.with_name(previous.name+'.sha256').write_text('incorrect checksum')
        with self.assertRaisesRegex(ValueError, 'checksum does not match'):
            self.build()
        self.assertTrue(previous.exists())
        self.assertFalse((self.testing/'Archive').exists())
        self.assertEqual(list(self.testing.glob('.telemetry-*.tmp')), [])

    def test_build_failure_does_not_archive_previous(self):
        previous = self.previous()
        (self.source/'bin/telemetry-live.dll').unlink()
        with self.assertRaises(SystemExit):
            self.build()
        self.assertTrue(previous.exists())
        self.assertFalse((self.testing/'Archive').exists())

    def test_cli_override_is_isolated_and_checksummed(self):
        previous = self.previous()
        isolated = self.source/'dist'
        subprocess.run([sys.executable, str(self.source/'source/package.py'), '--output', str(isolated)],
                       check=True, capture_output=True, text=True)
        self.assertTrue(previous.exists())
        self.assertFalse((self.testing/'Archive').exists())
        self.assert_checksum(isolated/'F1_Telemetry_v0.5.9-test.2.zip')

    def test_stable_default_preserves_testing_and_releases(self):
        previous = self.previous()
        self.set_version('0.5.9')
        archive = self.build()
        self.assertEqual(archive.parent, self.source/'dist')
        self.assertTrue(previous.exists())
        self.assertFalse((self.testing/'Archive').exists())
        self.assert_checksum(archive)

    def test_test_build_cannot_target_releases(self):
        with self.assertRaisesRegex(ValueError, 'must not write into Releases'):
            self.build(self.releases)
        self.assertEqual(list(self.releases.iterdir()), [self.releases/'keep.zip'])


if __name__ == '__main__':
    unittest.main()
