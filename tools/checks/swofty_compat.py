"""Prepare upstream validators once; report manifest, views and SDK checks separately."""
from pathlib import Path, PurePosixPath
import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import tomllib
import urllib.request
import zipfile

ROOT=Path(__file__).resolve().parents[2]
OUTPUT=ROOT/'.verify/swofty-compat'
REPOSITORY='Swofty-Developments/SwoftyLauncher'

def api(resource):
    headers={'User-Agent':'F1-Telemetry-Swofty-Compatibility','Accept':'application/vnd.github+json'}
    if os.environ.get('GH_TOKEN'):headers['Authorization']='Bearer '+os.environ['GH_TOKEN']
    request=urllib.request.Request('https://api.github.com/repos/'+REPOSITORY+'/'+resource,headers=headers)
    with urllib.request.urlopen(request,timeout=60) as response:return json.load(response)

def run(args,**kwargs):
    return subprocess.run([str(a) for a in args],cwd=ROOT,check=True,timeout=600,**kwargs)

def compiler_environment():
    vswhere=Path(os.environ.get('ProgramFiles(x86)',r'C:\Program Files (x86)'))/'Microsoft Visual Studio/Installer/vswhere.exe'
    install=subprocess.check_output([str(vswhere),'-latest','-products','*','-requires','Microsoft.VisualStudio.Component.VC.Tools.x86.x64','-property','installationPath'],text=True).strip()
    dev=Path(install)/'Common7/Tools/VsDevCmd.bat'
    result=subprocess.run(f'cmd.exe /d /s /c "call "{dev}" -arch=x64 -host_arch=x64 >nul && set"',capture_output=True,text=True,check=True)
    env={k.upper():v for k,v in os.environ.items()}
    env.update((line.split('=',1)[0].upper(),line.split('=',1)[1]) for line in result.stdout.splitlines() if '=' in line and not line.startswith('='))
    return env

def sdk_report(header,folder):
    folder.mkdir(parents=True,exist_ok=True)
    env=compiler_environment()
    compiler=Path(env['VCTOOLSINSTALLDIR'])/'bin/Hostx64/x64/cl.exe'
    run([compiler,'/nologo','/std:c++20','/EHsc','/O2','/MT','/W4','/WX','/I'+str(header.parent),ROOT/'tools/checks/sdk_contract.cpp','/Fo'+str(folder/'probe.obj'),'/Fe'+str(folder/'probe.exe')],env=env)
    return json.loads(subprocess.check_output([str(folder/'probe.exe')],text=True))

def compare_sdk(baseline,upstream):
    assert upstream['api_version']>=baseline['api_version'],'Upstream no longer supports the required API version'
    differences={k:(v,upstream.get(k)) for k,v in baseline.items() if k!='api_version' and upstream.get(k)!=v}
    assert not differences,'Consumed native SDK contract changed: '+json.dumps(differences)

def prepare(no_install=False,offline=False):
    config=json.loads((ROOT/'tools/checks/swofty_versions.json').read_text())
    assert config['repository']==REPOSITORY
    minimum=config['minimum']
    manifest=json.loads((ROOT/'swofty.json').read_text())
    assert manifest['compat']['launcher']=='>='+minimum['tag'].removeprefix('v'),'Minimum pin must match manifest support'
    release=api('releases/latest')
    assert not release['draft'] and not release['prerelease']
    def version(tag):
        match=re.fullmatch(r'v?(\d+)\.(\d+)\.(\d+)',tag)
        assert match,'Unexpected release tag'
        return tuple(map(int,match.groups()))
    assert version(release['tag_name'])>=version(minimum['tag'])
    from urllib.parse import quote
    latest_sha=api('commits/'+quote(release['tag_name'],safe=''))['sha']
    versions={'minimum':minimum,'latest':dict(tag=release['tag_name'],commit=latest_sha)}
    OUTPUT.mkdir(parents=True,exist_ok=True)
    (OUTPUT/'versions.json').write_text(json.dumps(versions,indent=2)+'\n')
    baseline=sdk_report(ROOT/'native/swofty_api.h',ROOT/'.verify/sdk-baseline')
    (OUTPUT/'sdk-baseline.json').write_text(json.dumps(baseline,indent=2)+'\n')
    compiled={};installed=set()
    for label,item in versions.items():
        sha=item['commit'];assert re.fullmatch('[0-9a-f]{40}',sha)
        target=OUTPUT/label;target.mkdir(exist_ok=True)
        if sha in compiled:
            for name in ('swofty_api.h','validator.exe','compile-error.txt'):
                previous=compiled[sha]/name
                if previous.exists():shutil.copy2(previous,target/name)
            print(f'{label}: reusing validator for identical commit {sha}',flush=True)
            continue
        archive=OUTPUT/(sha+'.zip')
        if not archive.exists():
            # Public archive downloads receive no authentication headers.
            url='https://codeload.github.com/'+REPOSITORY+'/zip/'+sha
            with urllib.request.urlopen(url,timeout=120) as response,archive.open('wb') as out:shutil.copyfileobj(response,out)
        upstream=ROOT/'.verify/swofty-sources'/sha
        upstream.mkdir(parents=True,exist_ok=True)
        with zipfile.ZipFile(archive) as z:
            for entry in z.infolist():
                parts=PurePosixPath(entry.filename).parts
                relative=Path(*parts[1:])
                destination=upstream/relative
                assert not PurePosixPath(entry.filename).is_absolute() and '..' not in parts and destination.resolve().is_relative_to(upstream.resolve())
                if not relative.parts or entry.is_dir():continue
                destination.parent.mkdir(parents=True,exist_ok=True)
                destination.write_bytes(z.read(entry))
        shutil.copy2(upstream/'packages/sdk/include/swofty_api.h',target/'swofty_api.h')
        example=upstream/'packages/addon-api/examples/telemetry_contract.rs'
        example.parent.mkdir(exist_ok=True)
        shutil.copy2(ROOT/'tools/checks/native_api.rs',example)
        channel=tomllib.loads((upstream/'rust-toolchain.toml').read_text())['toolchain']['channel']
        assert re.fullmatch(r'\d+\.\d+\.\d+',channel),'Expected an explicitly pinned upstream Rust toolchain'
        env=compiler_environment()
        env['CARGO_TARGET_DIR']=str(ROOT/'.verify/swofty-target')
        if not no_install and channel not in installed:
            run(['rustup','toolchain','install',channel,'--profile','minimal'],env=env)
            installed.add(channel)
        command=['cargo','+'+channel,'build','--locked','--manifest-path',upstream/'Cargo.toml','-p','swofty-addon','--example','telemetry_contract']
        if offline:command.insert(3,'--offline')
        print(f'Preparing {label}: {item["tag"]} at {sha}',flush=True)
        result=subprocess.run([str(a) for a in command],cwd=ROOT,env=env,capture_output=True,text=True,timeout=600)
        if result.returncode:
            (target/'validator.exe').unlink(missing_ok=True)
            (target/'compile-error.txt').write_text(result.stdout+result.stderr,encoding='utf-8')
            print(f'{label}: upstream validator build failed; its individual manifest/view checks will fail with saved compiler output',flush=True)
        else:
            (target/'compile-error.txt').unlink(missing_ok=True)
            shutil.copy2(Path(env['CARGO_TARGET_DIR'])/'debug/examples/telemetry_contract.exe',target/'validator.exe')
        compiled[sha]=target

def check(label,kind):
    target=OUTPUT/label
    versions=json.loads((OUTPUT/'versions.json').read_text())
    print(f'{label}: {versions[label]["tag"]} ({versions[label]["commit"]})',flush=True)
    if kind=='sdk':
        header=target/'swofty_api.h'
        if label=='minimum':
            assert hashlib.sha256(header.read_bytes()).digest()==hashlib.sha256((ROOT/'native/swofty_api.h').read_bytes()).digest(),'Vendored SDK differs from minimum release'
        baseline=json.loads((OUTPUT/'sdk-baseline.json').read_text())
        compare_sdk(baseline,sdk_report(header,ROOT/'.verify'/('sdk-'+label)))
        print('PASS: consumed native field offsets, widths, callback signatures and constants remain compatible',flush=True)
    else:
        if (target/'compile-error.txt').exists():raise RuntimeError((target/'compile-error.txt').read_text(encoding='utf-8'))
        argument=ROOT if kind=='manifest' else ROOT/'.verify/native-views.jsonl'
        run([target/'validator.exe',kind,argument])

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command',choices=('prepare','check'))
    parser.add_argument('--release',choices=('minimum','latest'))
    parser.add_argument('--kind',choices=('manifest','views','sdk'))
    parser.add_argument('--no-install',action='store_true')
    parser.add_argument('--offline',action='store_true')
    args=parser.parse_args()
    if args.command=='prepare':prepare(args.no_install,args.offline)
    else:
        assert args.release and args.kind
        check(args.release,args.kind)
