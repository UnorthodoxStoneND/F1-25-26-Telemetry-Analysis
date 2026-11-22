"""Collect redistribution notices for the pinned desktop Go dependencies."""
import json
import os
from pathlib import Path
import subprocess

ROOT=Path(__file__).resolve().parents[1]

def main():
    env=dict(os.environ,GOTOOLCHAIN='local',GOPATH=str(ROOT/'source/.gomodcache/gopath'),GOMODCACHE=str(ROOT/'source/.gomodcache'))
    raw=subprocess.check_output(['go','mod','download','-json','all'],cwd=ROOT/'source',env=env,text=True)
    decoder=json.JSONDecoder();modules=[]
    while raw.strip():
        item,at=decoder.raw_decode(raw.lstrip());modules.append(item);raw=raw.lstrip()[at:]
    texts=['Desktop dependency redistribution notices\n\nPinned versions are recorded in source/go.mod and source/go.sum.\nIncludes module, font and embedded C-library licences.\n']
    for module in sorted(modules,key=lambda m:m['Path']):
        directory=Path(module['Dir'])
        licences=sorted(p for p in directory.rglob('*') if p.is_file() and (p.name.upper().startswith(('LICENSE','LICENCE','COPYING','NOTICE','OFL')) or p.name.lower()=='copying.txt'))
        if not licences: raise RuntimeError('Missing licence for '+module['Path'])
        texts.append('\n'+'='*76+'\n'+module['Path']+' '+module['Version']+'\n')
        for licence in licences:
            texts.append('\n--- '+licence.relative_to(directory).as_posix()+' ---\n'+licence.read_text(encoding='utf-8',errors='replace'))
    texts.append((ROOT/'native/DESKTOP-TOOLCHAIN-LICENSES.txt').read_text(encoding='utf-8'))
    (ROOT/'DESKTOP_LICENSES.txt').write_text('\n'.join(texts),encoding='utf-8')

if __name__=='__main__':main()
