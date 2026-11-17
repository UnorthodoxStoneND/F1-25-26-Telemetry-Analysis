"""Execute one individually reported CI check against shared build artifacts."""
from pathlib import Path
import argparse
import os
import re
import subprocess
import sys

ROOT=Path(__file__).resolve().parents[2]
NATIVE=ROOT/'.verify/native'
DEDICATED={'TestReleasedAndFutureLauncherHandshake','TestBoundedLiveMemory','TestNativeABI'}

def run(args,env=None):
    subprocess.run([str(a) for a in args],cwd=ROOT,env=env,check=True,timeout=180)

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('check',choices=('unit','static','protocol','memory','policy','readers','publisher','helper','integrity','privacy'))
    args=parser.parse_args()
    test=ROOT/'.verify/go-tests.exe'
    env=dict(os.environ,TELEMETRY_NATIVE_VIEWS=str(ROOT/'.verify/native-views.jsonl'))
    if args.check=='unit':
        listed=subprocess.check_output([str(test),'-test.list','^Test'],cwd=ROOT,text=True)
        names=[n for n in listed.splitlines() if n.startswith('Test') and n not in DEDICATED]
        assert names and DEDICATED<=set(listed.splitlines()),'Dedicated tests must be present and partitioned'
        run([test,'-test.v','-test.run','^('+'|'.join(map(re.escape,names))+')$'],env)
    elif args.check in ('protocol','memory'):
        name='TestReleasedAndFutureLauncherHandshake' if args.check=='protocol' else 'TestBoundedLiveMemory'
        run([test,'-test.v','-test.run','^'+name+'$'])
    elif args.check=='static':run(['go','-C','source','vet','./...'])
    elif args.check=='policy':run([sys.executable,'tools/checks/live_policy.py'])
    elif args.check=='readers':
        run([NATIVE/'readers_test.exe',NATIVE/'frame.bin'])
        run([NATIVE/'binding_test.exe'])
        run([NATIVE/'live_api_test.exe'])
        run([test,'-test.v','-test.run','^TestNativeABI$'],dict(os.environ,TELEMETRY_ABI_FRAME=str(NATIVE/'frame.bin')))
    elif args.check=='publisher':
        run([NATIVE/'runtime_check.exe', ROOT/'bin/telemetry-live.dll'])
        missing=subprocess.run([str(NATIVE/'runtime_check.exe'),str(ROOT/'bin/telemetry-live.dll'),'telemetry-intentionally-missing-runtime.dll'],capture_output=True,text=True)
        assert missing.returncode==2 and 'Install or repair' in missing.stderr,missing
        for mode in ([],['verified'],['current']):
            run([NATIVE/'publisher_test.exe',NATIVE/'swofty_core.dll',ROOT/'bin/telemetry-live.dll']+mode)
    elif args.check=='helper':run([sys.executable,'tools/checks/lifecycle_check.py'])
    elif args.check=='privacy':
        run([sys.executable,'tools/checks/package_privacy_test.py'])
        run([sys.executable,'tools/checks/package_check.py','--check','privacy'])
    else:run([sys.executable,'tools/checks/package_check.py','--check',args.check])

if __name__=='__main__':main()
