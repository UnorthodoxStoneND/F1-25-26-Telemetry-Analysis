"""Validate the installable ZIP and public-source privacy independently."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tempfile
import zipfile

ROOT=Path(__file__).resolve().parents[2]
EMAIL=re.compile(r'[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}')
PERSONAL_PATH=re.compile(r'[A-Za-z]:[\\/]Users[\\/][^\\/\s]+',re.IGNORECASE)
def privacy(name,raw):
    for encoding in ('utf-8','utf-16-le'):
        text=raw.decode(encoding,errors='replace')
        assert not EMAIL.search(text) and not PERSONAL_PATH.search(text), f'Email or personal path in {name}'
def sources():
    # Also supports an isolated, non-Git build directory.
    result=subprocess.run(['git','-C',str(ROOT),'ls-files','-z'],capture_output=True)
    if result.returncode==0:
        return [ROOT/name for name in result.stdout.decode().split('\0') if name]
    return [p for p in ROOT.rglob('*') if p.is_file() and not any(part.startswith('.') or part in ('bin','dist','__pycache__') for part in p.relative_to(ROOT).parts)]
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--archive',type=Path);p.add_argument('--check',choices=('all','integrity','privacy'),default='all');args=p.parse_args()
    manifest=json.loads((ROOT/'swofty.json').read_text(encoding='utf-8-sig'));version=manifest['version']
    archive=args.archive or ROOT/'dist'/f'F1_Telemetry_v{version}.zip'
    with zipfile.ZipFile(archive) as z:
        if args.check in ('all','integrity'):
            assert f'const addonVersion = "{version}"' in (ROOT/'source/main.go').read_text(encoding='utf-8')
            assert f'## {version} ' in (ROOT/'CHANGELOG.md').read_text(encoding='utf-8-sig')
            assert archive.name in (ROOT/'README.md').read_text(encoding='utf-8')
            assert manifest['launcher']['sections']==[{'id':'telemetry','title':'Live Telemetry','icon':'activity'}]
            assert manifest['operations']['native']==[{'dll':'bin/telemetry-live.dll'}]
            assert manifest['compat']['launcher']=='>=1.9.55'
            assert manifest['launcher']['program']['windows']=='bin/telemetry-live.exe'
            expected={'swofty.json','README.md','CHANGELOG.md','LICENSE','THIRD_PARTY_NOTICES.md','source/go.mod','bin/telemetry-live.exe','bin/telemetry-live.dll','native/SDK-LICENSE.txt','native/swofty_api.h'}
            names=z.namelist();assert z.testzip() is None and len(names)==len(set(names)) and expected<=set(names)
            for name in names:
                path=PurePosixPath(name);assert not path.is_absolute() and '..' not in path.parts and '\\' not in name
                assert name in expected or path.parent==PurePosixPath('source') and path.suffix in {'.go','.py'} or path.parent==PurePosixPath('native') and path.suffix in {'.h','.cpp','.py'}
                assert z.read(name)==(ROOT/name).read_bytes(),f'Stale ZIP member: {name}'
            assert json.loads(z.read('swofty.json'))==manifest
        if args.check in ('all','privacy'):
            for name in z.namelist():privacy(name,z.read(name))
            for file in sources():privacy(file.name,file.read_bytes())
            print('PASS: public source and ZIP contain no email addresses or personal Windows paths')
    if args.check in ('all','integrity'):
        with tempfile.TemporaryDirectory() as temp:
            subprocess.run([sys.executable,str(ROOT/'source/package.py'),'--output',temp],check=True)
            assert hashlib.sha256((Path(temp)/archive.name).read_bytes()).digest()==hashlib.sha256(archive.read_bytes()).digest()
        print('PASS: complete reproducible live-only ZIP, version consistency, no recordings or images')
if __name__=='__main__':main()
