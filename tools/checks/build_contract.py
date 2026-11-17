"""Check release PE architecture, subsystem, runtime imports and retained debug data."""
from pathlib import Path
import struct

ROOT=Path(__file__).resolve().parents[2]

def inspect(path):
    b=path.read_bytes()
    u16=lambda p:struct.unpack_from('<H',b,p)[0]
    u32=lambda p:struct.unpack_from('<I',b,p)[0]
    pe=u32(60); opt=pe+24
    assert b[:2]==b'MZ' and b[pe:pe+4]==b'PE\0\0' and u16(pe+4)==0x8664 and u16(opt)==0x20b,path.name
    sections=[]
    strings=u32(pe+12)+18*u32(pe+16)
    for i in range(u16(pe+6)):
        p=opt+u16(pe+20)+40*i
        name=b[p:p+8].split(b'\0')[0].decode()
        if name.startswith('/'):
            s=strings+int(name[1:]);name=b[s:b.index(b'\0',s)].decode()
        sections.append((name,u32(p+12),u32(p+8),u32(p+20),u32(p+16)))
    def offset(rva):
        for _,at,size,raw,raw_size in sections:
            if at<=rva<at+max(size,raw_size):return raw+rva-at
        raise AssertionError('Unmapped PE RVA')
    imports=[]
    p=offset(u32(opt+120))
    while any(b[p:p+20]):
        name=offset(u32(p+12));imports.append(b[name:b.index(b'\0',name)].decode().lower());p+=20
    return dict(subsystem=u16(opt+68),flags=u16(opt+70),imports=imports,sections=[s[0] for s in sections])

def main():
    exe=inspect(ROOT/'bin/telemetry-live.exe')
    dll=inspect(ROOT/'bin/telemetry-live.dll')
    assert exe['subsystem']==3,'Helper must retain the tested console subsystem'
    assert '.symtab' in exe['sections'] and any(s.startswith('.zdebug_') for s in exe['sections']),'Helper symbols/compressed DWARF missing'
    assert 'vcruntime140.dll' in dll['imports'],'Native DLL must use /MD'
    assert any(s.startswith('api-ms-win-crt-') for s in dll['imports']),'Universal CRT dependency missing'
    assert all(s=='kernel32.dll' or s=='vcruntime140.dll' or s.startswith('api-ms-win-crt-') for s in dll['imports']),dll['imports']
    assert all(x['flags']&0x140==0x140 for x in (exe,dll)),'ASLR/NX must stay enabled'
    readme=(ROOT/'README.md').read_text()
    assert 'same Microsoft Visual C++ v14 x64 runtime family as Swofty Launcher' in readme and 'https://aka.ms/vc14/vc_redist.x64.exe' in readme
    assert '14.44' not in readme,'Do not impose an add-on-only runtime version cutoff'
    print('PASS: x64 console helper with symbols/compressed DWARF; /MD native runtime contract; ASLR/NX; documented x64 prerequisite')

if __name__=='__main__':main()
