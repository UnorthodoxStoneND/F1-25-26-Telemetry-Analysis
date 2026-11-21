"""Validate the installable ZIP and public-source privacy independently."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import struct
import subprocess
import sys
import tempfile
import zipfile
import zlib

ROOT=Path(__file__).resolve().parents[2]
EMAIL=re.compile(r'[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}')
PERSONAL_PATH=re.compile(r'[A-Za-z]:[\\/]Users[\\/][^\\/\s]+',re.IGNORECASE)

# Audited upstream licence text can contain public copyright contacts. Require
# its exact content hash; a new contact or edited notice is still rejected.
PUBLIC_LICENSE_HASHES={
    'DESKTOP_LICENSES.txt':'3ab49dfd065059ec7a9c9cefccd6d8310578698b46376fd2c1612e16e7d3d7ad',
    'DESKTOP-TOOLCHAIN-LICENSES.txt':'cae9f847a4b50291ae0fa19f00a6fb1152bd9e6cd663c4ef88d7cb6aec97a172',
}

def public_desktop_assets(raw):
    # Noto font outlines happen to contain email-shaped binary bytes. Mask only
    # whole, hash-verified upstream TTF assets, never arbitrary matching text.
    fonts=[
        ('00010000001201000004002047444546fefbf2ab0000012c000005f247504f53',470472,'35600501daadd3e9030ee6ed8098fd3c7f31bfd898ca58da8edf793a3231c48b'),
        ('00010000001201000004002047444546f0d7dde70000012c000005c047504f53',455188,'6b04c8dd65af6b73eb4279472ed1580b29102d6496a377340e80a40cdb3b22c9'),
    ]
    for prefix,size,digest in fonts:
        start=0;prefix=bytes.fromhex(prefix)
        while (at:=raw.find(prefix,start))>=0:
            start=at+len(prefix)
            if hashlib.sha256(raw[at:at+size]).hexdigest()==digest:
                raw=raw[:at]+b'\0'*size+raw[at+size:]
    # BurntSushi/toml 1.4.0 embeds this public Windows-path help example.
    # Keep the entire surrounding sentence exact, including both examples.
    example=bytes.fromhex('496620796f7527726520747279696e6720746f2061646420612057696e646f777320706174682028652e672e2022433a5c55736572735c6d617274696e2229207468656e207573696e6720272f270a696e7374656164206f6620275c272077696c6c20757375616c6c7920616c736f20776f726b3a2022433a2f55736572732f6d617274696e222e0a')
    return raw.replace(example,b'\0'*len(example))

def privacy_pe_debug(raw):
    """Scan real DWARF text instead of incidental matches in its compressed bytes."""
    try:
        pe=struct.unpack_from('<I',raw,60)[0]
        if raw[pe:pe+4]!=b'PE\0\0':return raw
        count=struct.unpack_from('<H',raw,pe+6)[0]
        if count>96:return raw
        table=pe+24+struct.unpack_from('<H',raw,pe+20)[0]
        strings=struct.unpack_from('<I',raw,pe+12)[0]+18*struct.unpack_from('<I',raw,pe+16)[0]
        scanned=bytearray(raw);decoded=[]
        for index in range(count):
            entry=table+40*index
            name=raw[entry:entry+8].split(b'\0')[0]
            if name.startswith(b'/'):
                at=strings+int(name[1:])
                name=raw[at:raw.index(b'\0',at)]
            if not name.startswith(b'.zdebug_'):continue
            size,offset=struct.unpack_from('<II',raw,entry+16)
            if offset+size>len(raw) or size<12 or raw[offset:offset+4]!=b'ZLIB':continue
            expected=int.from_bytes(raw[offset+4:offset+12],'big')
            if expected>64*1024*1024:continue
            compressed=raw[offset+12:offset+size]
            stream=zlib.decompressobj()
            text=stream.decompress(compressed,expected+1)
            if not stream.eof or len(text)!=expected:continue
            consumed=len(compressed)-len(stream.unused_data)
            # Keep headers and trailing section bytes in the scan, including padding.
            scanned[offset+12:offset+12+consumed]=b'\0'*consumed
            decoded.append(text)
        for text in decoded:
            # Keep UTF-16 payloads aligned and separate from preceding bytes.
            scanned.extend(b'\0'*(2-len(scanned)%2))
            scanned.extend(text)
        return bytes(scanned)
    except (ValueError,struct.error,zlib.error):
        return raw
def privacy_elf_debug(raw):
    """Decode ELF64 compressed DWARF from the native Linux Go helper."""
    try:
        if raw[:6]!=b'\x7fELF\x02\x01':return raw
        table=struct.unpack_from('<Q',raw,40)[0]
        stride,count,names=struct.unpack_from('<HHH',raw,58)
        if stride<64 or count>128 or names>=count:return raw
        names_at,names_size=struct.unpack_from('<QQ',raw,table+names*stride+24)
        labels=raw[names_at:names_at+names_size]
        scanned=bytearray(raw);decoded=[]
        for index in range(count):
            entry=table+index*stride
            label=struct.unpack_from('<I',raw,entry)[0]
            name=labels[label:labels.index(b'\0',label)]
            flags=struct.unpack_from('<Q',raw,entry+8)[0]
            offset,size=struct.unpack_from('<QQ',raw,entry+24)
            if not name.startswith(b'.debug_') or not flags&0x800:continue
            if size<24 or offset+size>len(raw):continue
            kind,_,expected,_=struct.unpack_from('<IIQQ',raw,offset)
            if kind!=1 or expected>64*1024*1024:continue
            compressed=raw[offset+24:offset+size]
            stream=zlib.decompressobj();text=stream.decompress(compressed,expected+1)
            if not stream.eof or len(text)!=expected:continue
            consumed=len(compressed)-len(stream.unused_data)
            scanned[offset+24:offset+24+consumed]=b'\0'*consumed
            decoded.append(text)
        for text in decoded:
            scanned.extend(b'\0'*(2-len(scanned)%2));scanned.extend(text)
        return bytes(scanned)
    except (ValueError,IndexError,struct.error,zlib.error):return raw

def privacy_scan_bytes(raw):
    """Ignore only the public Trufo CA contact in PNG provenance certificate DNs.

    This changes the temporary scan buffer, never the artwork or its provenance.
    All other certificate fields, image chunks and ordinary files remain scanned.
    """
    if raw.startswith(b'MZ'):return privacy_pe_debug(public_desktop_assets(raw))
    if raw.startswith(b'\x7fELF'):return privacy_elf_debug(public_desktop_assets(raw))
    if not raw.startswith(b'\x89PNG\r\n\x1a\n'):
        return raw
    contact='@'.join(('ca','trufo.ai')).encode('ascii')
    field=bytes.fromhex('301806092a864886f70d010901160b')+contact
    scanned=bytearray(raw)
    offset=8
    while offset+12 <= len(raw):
        length=struct.unpack('>I',raw[offset:offset+4])[0]
        end=offset+8+length
        if end+4 > len(raw):
            break
        if raw[offset+4:offset+8] == b'caBX':
            start=offset+8
            while (match:=raw.find(field,start,end)) != -1:
                begin=match+len(field)-len(contact)
                scanned[begin:begin+len(contact)]=b' '*len(contact)
                start=match+len(field)
        offset=end+4
    return bytes(scanned)

def privacy(name,raw):
    public_notice=PUBLIC_LICENSE_HASHES.get(PurePosixPath(name).name)==hashlib.sha256(raw).hexdigest()
    raw=privacy_scan_bytes(raw)
    for encoding in ('utf-8','utf-16-le'):
        text=raw.decode(encoding,errors='replace')
        assert (public_notice or not EMAIL.search(text)) and not PERSONAL_PATH.search(text), f'Email or personal path in {name}'
def sources():
    # Also supports an isolated, non-Git build directory.
    result=subprocess.run(['git','-C',str(ROOT),'ls-files','--cached','--others','--exclude-standard','-z'],capture_output=True)
    if result.returncode==0:
        return [ROOT/name for name in result.stdout.decode().split('\0') if name and (ROOT/name).is_file()]
    return [p for p in ROOT.rglob('*') if p.is_file() and not any(part.startswith('.') or part in ('bin','dist','__pycache__') for part in p.relative_to(ROOT).parts)]
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--archive',type=Path);p.add_argument('--check',choices=('all','integrity','privacy'),default='all');args=p.parse_args()
    manifest=json.loads((ROOT/'swofty.json').read_text(encoding='utf-8-sig'));version=manifest['version']
    default_dir=ROOT.parent/'Testing' if re.search(r'^\d+\.\d+\.\d+-test(?:[.+-]|$)',version) else ROOT/'dist'
    archive=args.archive or default_dir/f'F1_Telemetry_v{version}.zip'
    if args.check in ('all','integrity'):
        sidecar=archive.with_name(archive.name+'.sha256')
        expected_checksum=hashlib.sha256(archive.read_bytes()).hexdigest()+'  '+archive.name
        assert sidecar.read_text(encoding='ascii').strip()==expected_checksum,'ZIP checksum is missing or incorrect'
    with zipfile.ZipFile(archive) as z:
        if args.check in ('all','integrity'):
            subprocess.run([sys.executable,str(ROOT/'tools/checks/build_contract.py')],check=True)
            assert f'const addonVersion = "{version}"' in (ROOT/'source/main.go').read_text(encoding='utf-8')
            assert f'## {version} ' in (ROOT/'CHANGELOG.md').read_text(encoding='utf-8-sig')
            assert archive.name in (ROOT/'README.md').read_text(encoding='utf-8')
            assert manifest['launcher']['sections']==[{'id':'telemetry','title':'Live Telemetry','icon':'activity'}]
            assert manifest['operations']['native']==[{'dll':'bin/telemetry-live.dll'}]
            assert manifest['compat']['launcher']=='>=1.10.9'
            assert manifest['launcher']['program']['windows']=='bin/telemetry-live.exe'
            assert manifest['launcher']['program']['linux']=='bin/telemetry-live-linux'
            assert z.getinfo('bin/telemetry-live-linux').external_attr>>16&0o111
            assert manifest['license']=='MIT'
            assert z.read('LICENSE').decode().splitlines()[0]=='MIT License'
            assert 'Copyright (c) 2026 UnorthodoxStoneNDmodding' in z.read('LICENSE').decode()
            assert 'Permission is hereby granted, free of charge' in z.read('LICENSE').decode()
            assert 'THE SOFTWARE IS PROVIDED' in z.read('LICENSE').decode()
            for name in z.namelist():
                if name == 'native/swofty_game_types.hpp':
                    assert 'Required Notice: Copyright Swofty Pty Ltd' in z.read(name).decode()
                    assert 'LicenseRef-PolyForm-Noncommercial-1.0.0' in z.read(name).decode()
                elif name.startswith('native/') and PurePosixPath(name).suffix in {'.cpp','.h','.hpp'}:
                    assert 'SPDX-License-Identifier: MIT' in z.read(name).decode(), f'Incorrect source licence: {name}'
            assert b'BSD-3-Clause' in z.read('THIRD_PARTY_NOTICES.md')
            assert b'Apache-2.0 WITH LLVM-exception' in z.read('THIRD_PARTY_NOTICES.md')
            assert b'MIT License' in z.read('native/SDK-LICENSE.txt')
            expected={'swofty.json','README.md','CHANGELOG.md','LICENSE','THIRD_PARTY_NOTICES.md','DESKTOP_LICENSES.txt','bin/telemetry-live.exe','bin/telemetry-live.dll','bin/telemetry-live-linux','native/SDK-LICENSE.txt','native/DESKTOP-TOOLCHAIN-LICENSES.txt'}
            assert manifest['icon'] == 'icon.png'
            assert manifest['compat']['platforms'] == ['steam']
            assert manifest['description'].strip() and manifest['category'] == 'utility'
            assert manifest['repository'] == 'https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis'
            expected.add(manifest['icon'])
            expected.add('native/SWOFTY-LAYOUTS-LICENSE.txt')
            assert b'Required Notice: Copyright Swofty Pty Ltd' in z.read('native/SWOFTY-LAYOUTS-LICENSE.txt')
            artwork=z.read(manifest['icon'])
            assert 0 < len(artwork) <= 2_000_000 and artwork[:8] == b'\x89PNG\r\n\x1a\n'
            width,height=struct.unpack('>II',artwork[16:24])
            assert width > 0 and width == height, 'Browser icon must be a non-empty square'
            names=z.namelist();assert z.testzip() is None and len(names)==len(set(names)) and expected==set(names),'Only runtime files and required documentation may be shipped'
            for name in names:
                path=PurePosixPath(name);assert not path.is_absolute() and '..' not in path.parts and '\\' not in name
                assert name in expected
                assert z.read(name)==(ROOT/name).read_bytes(),f'Stale ZIP member: {name}'
            assert json.loads(z.read('swofty.json'))==manifest
        if args.check in ('all','privacy'):
            for name in z.namelist():privacy(name,z.read(name))
            for file in sources():privacy(file.name,file.read_bytes())
            print('PASS: no personal emails/paths; PE/ELF debug data scanned after decompression; public PNG CA contact limited to certificate DN fields')
    if args.check in ('all','integrity'):
        with tempfile.TemporaryDirectory() as temp:
            subprocess.run([sys.executable,str(ROOT/'source/package.py'),'--output',temp],check=True)
            assert hashlib.sha256((Path(temp)/archive.name).read_bytes()).digest()==hashlib.sha256(archive.read_bytes()).digest()
        print('PASS: complete reproducible live-only ZIP, version consistency, no recordings or generated telemetry images; browser icon included')
if __name__=='__main__':main()
