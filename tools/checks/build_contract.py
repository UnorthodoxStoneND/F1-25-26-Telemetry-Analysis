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
    resources={}
    resource_rva=u32(opt+128)
    if resource_rva:
        base=offset(resource_rva)
        def walk(relative, keys=()):
            assert len(keys)<=3, 'Unexpected resource depth'
            directory=base+relative
            for i in range(u16(directory+12)+u16(directory+14)):
                entry=directory+16+i*8
                key=u32(entry); target=u32(entry+4)
                if target&0x80000000:
                    walk(target&0x7fffffff,keys+(key,))
                else:
                    data=base+target
                    at=offset(u32(data)); size=u32(data+4)
                    resources[keys+(key,)]=b[at:at+size]
        walk(0)
    # CGO desktop helpers use MinGW's COFF symbol table; the former internally
    # linked helper used a .symtab section. Both must retain inspectable symbols.
    coff_symbols=u32(pe+16) if u32(pe+12)>0 and u32(pe+12)+18*u32(pe+16)<=len(b) else 0
    return dict(resources=resources,subsystem=u16(opt+68),flags=u16(opt+70),imports=imports,sections=[s[0] for s in sections],coff_symbols=coff_symbols)

def main():
    elf=(ROOT/'bin/telemetry-live-linux').read_bytes()
    assert elf[:6]==b'\x7fELF\x02\x01' and struct.unpack_from('<H',elf,18)[0]==62,'Linux helper must be x86-64 ELF'
    assert struct.unpack_from('<H',elf,16)[0]==3,'Linux helper must be PIE for address randomization'
    program_table=struct.unpack_from('<Q',elf,32)[0]
    program_stride,program_count=struct.unpack_from('<HH',elf,54)
    programs=[struct.unpack_from('<IIQQQQQQ',elf,program_table+i*program_stride) for i in range(program_count)]
    assert not any(kind==1 and flags&3==3 for kind,flags,*_ in programs),'Linux helper must not have writable executable segments'
    stacks=[flags for kind,flags,*_ in programs if kind==0x6474e551]
    assert stacks and all(flags&1==0 for flags in stacks),'Linux helper must have a non-executable stack'
    section_table=struct.unpack_from('<Q',elf,40)[0]
    section_stride,section_count,name_index=struct.unpack_from('<HHH',elf,58)
    names_at,names_size=struct.unpack_from('<QQ',elf,section_table+name_index*section_stride+24)
    names=elf[names_at:names_at+names_size]
    debug_sections=set()
    for i in range(section_count):
        entry=section_table+i*section_stride
        name_at=struct.unpack_from('<I',elf,entry)[0]
        name=names[name_at:names.index(b'\0',name_at)].decode()
        if name.startswith('.debug_'):
            assert not struct.unpack_from('<Q',elf,entry+8)[0]&0x800,'Linux debug sections must stay uncompressed'
            debug_sections.add(name)
    assert {'.debug_info','.debug_line'}<=debug_sections,'Linux helper must retain DWARF'
    exe=inspect(ROOT/'bin/telemetry-live.exe')
    dll=inspect(ROOT/'bin/telemetry-live.dll')
    icons=[data for key,data in exe['resources'].items() if key[0]==3]
    groups=[data for key,data in exe['resources'].items() if key[0]==14]
    assert len(icons)==4 and len(groups)==1, 'Missing Windows executable icon resources'
    assert struct.unpack_from('<HHH',groups[0])==(0,1,4), 'Icon group must include four sizes'
    desktop_icon=(ROOT/'source/retro_icon.png').read_bytes()
    assert desktop_icon in icons, 'Executable and window icon differ'
    assert desktop_icon in elf, 'Linux window icon missing'

    assert exe['subsystem']==3,'Helper must retain the tested console subsystem'
    assert ('.symtab' in exe['sections'] or exe['coff_symbols']>0) and {'.debug_info','.debug_line'}<=set(exe['sections']),'Helper symbols/uncompressed DWARF missing'
    assert not any(s.startswith('.zdebug_') for s in exe['sections']),'Windows release debug sections must stay uncompressed'
    assert set(exe['imports']) <= {'gdi32.dll','kernel32.dll','msvcrt.dll','opengl32.dll','shell32.dll','user32.dll'},'Unexpected desktop runtime DLL'
    assert 'vcruntime140.dll' in dll['imports'],'Native DLL must use /MD'
    assert any(s.startswith('api-ms-win-crt-') for s in dll['imports']),'Universal CRT dependency missing'
    assert all(s=='kernel32.dll' or s=='vcruntime140.dll' or s.startswith('api-ms-win-crt-') for s in dll['imports']),dll['imports']
    assert all(x['flags']&0x140==0x140 for x in (exe,dll)),'ASLR/NX must stay enabled'
    readme=(ROOT/'README.md').read_text()
    assert 'same Microsoft Visual C++ v14 x64 runtime family as Swofty Launcher' in readme and 'https://aka.ms/vc14/vc_redist.x64.exe' in readme
    assert '14.44' not in readme,'Do not impose an add-on-only runtime version cutoff'
    print('PASS: x64 desktop helper with uncompressed DWARF/system graphics imports; Linux PIE/non-executable stack/uncompressed DWARF; /MD native runtime contract; ASLR/NX; documented x64 prerequisite')

if __name__=='__main__':main()
