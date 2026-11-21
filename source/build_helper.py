"""Build the desktop-enabled helper (one process) for Windows or Linux.

Windows hosts use Ubuntu WSL with MinGW for Windows and GCC/X11/OpenGL for
Linux. Linux hosts use their local compiler. No browser runtime is bundled.
"""
import argparse
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]

def build(target):
    output = '../bin/telemetry-live.exe' if target == 'windows' else '../bin/telemetry-live-linux'
    (ROOT/'bin').mkdir(exist_ok=True)
    settings = dict(CGO_ENABLED='1', GOOS=target, GOARCH='amd64', GOTOOLCHAIN=os.environ.get('RETRO_GO_TOOLCHAIN','go1.24.5'))
    args = ['build','-tags','desktop','-buildvcs=false','-trimpath']
    if target == 'windows':
        settings['CC'] = 'x86_64-w64-mingw32-gcc'
        args += ['-ldflags=-compressdwarf=false -extldflags=-static-libgcc']
    else:
        args += ['-buildmode=pie','-ldflags=-compressdwarf=false']
    args += ['-o',output,'.']
    def run(program, *arguments, env=None):
        if os.name == 'nt':
            path = subprocess.check_output(['wsl','-d','Ubuntu','--','wslpath','-u',(ROOT/'source').as_posix()],text=True).strip()
            # env --chdir avoids shell quoting and supports workspace spaces.
            command = ['wsl','-d','Ubuntu','--','env','--chdir='+path,
                       *[f'{k}={v}' for k,v in (env or {}).items()], program, *arguments]
            subprocess.run(command,check=True)
        else:
            subprocess.run([program,*arguments],cwd=ROOT/'source',env={**os.environ,**(env or {})},check=True)

    run('go','run','generate_icon.go',env={'GOTOOLCHAIN': settings['GOTOOLCHAIN']})
    resource = ROOT/'source/icon_windows_amd64.syso'
    try:
        if target == 'windows':
            run('x86_64-w64-mingw32-windres','-i','../.verify/helper-icon.rc','-o',resource.name,'-O','coff')
        run('go',*args,env=settings)
    finally:
        if target == 'windows':
            resource.unlink(missing_ok=True)

if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target',choices=['windows','linux','both'],default='both')
    selected=parser.parse_args().target
    for target in (['windows','linux'] if selected=='both' else [selected]): build(target)
