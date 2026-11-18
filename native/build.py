"""Build the native add-on and run its host tests (Visual Studio C++ x64)."""
from pathlib import Path
import argparse
import os
import subprocess

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--tests-only', action='store_true')
parser.add_argument('--build-only', action='store_true', help='Compile CI artifacts without running tests')
args = parser.parse_args()
program_files = Path(os.environ.get('ProgramFiles(x86)', r'C:\Program Files (x86)'))
vswhere = program_files / 'Microsoft Visual Studio/Installer/vswhere.exe'
install = subprocess.check_output([str(vswhere), '-latest', '-products', '*', '-requires',
                                  'Microsoft.VisualStudio.Component.VC.Tools.x86.x64', '-property', 'installationPath'], text=True).strip()
if not install:
    raise SystemExit('Install Visual Studio Build Tools with Desktop development with C++.')
dev = Path(install) / 'Common7/Tools/VsDevCmd.bat'
result = subprocess.run(f'cmd.exe /d /s /c "call "{dev}" -arch=x64 -host_arch=x64 >nul && set"', capture_output=True, text=True, check=True)
env = {k.upper(): v for k, v in os.environ.items()}
env.update((line.split('=', 1)[0].upper(), line.split('=', 1)[1]) for line in result.stdout.splitlines() if '=' in line and not line.startswith('='))
compiler = Path(env['VCTOOLSINSTALLDIR']) / 'bin/Hostx64/x64/cl.exe'
output = root / '.verify/native'
output.mkdir(parents=True, exist_ok=True)
(root / 'bin').mkdir(exist_ok=True)
common = [str(compiler), '/nologo', '/std:c++20', '/EHsc', '/O2', '/MT', '/W4', '/WX', '/I'+str(root/'native')]
if not args.tests_only:
    dll_flags = [flag for flag in common if flag != '/MT'] + ['/MD']
    subprocess.run(dll_flags + ['/LD', str(root/'native/telemetry.cpp'), '/Fo'+str(output/'telemetry.obj'),
                            '/link', '/Brepro', '/OUT:'+str(root/'bin/telemetry-live.dll'),
                            '/IMPLIB:'+str(output/'telemetry-live.lib')], env=env, cwd=output, check=True)
subprocess.run(common + [str(root/'native/readers_test.cpp'), '/Fo'+str(output/'readers_test.obj'),
                        '/Fe'+str(output/'readers_test.exe'), '/link', '/Brepro'], env=env, cwd=output, check=True)
if not args.build_only:
    subprocess.run([str(output/'readers_test.exe'), str(output/'frame.bin')], check=True, timeout=30)
subprocess.run(common + [str(root/'native/binding_test.cpp'), '/Fo'+str(output/'binding_test.obj'),
                        '/Fe'+str(output/'binding_test.exe'), '/link', '/Brepro'], env=env, cwd=output, check=True)
if not args.build_only:
    subprocess.run([str(output/'binding_test.exe')], check=True, timeout=30)
subprocess.run(common + [str(root/'native/live_api_test.cpp'), '/Fo'+str(output/'live_api_test.obj'),
                        '/Fe'+str(output/'live_api_test.exe'), '/link', '/Brepro'], env=env, cwd=output, check=True)
if not args.build_only:
    subprocess.run([str(output/'live_api_test.exe'),str(output/'badges.bin')], check=True, timeout=30)
if not args.tests_only:
    subprocess.run(common + [str(root/'native/runtime_check.cpp'), '/Fo'+str(output/'runtime_check.obj'),
                            '/Fe'+str(output/'runtime_check.exe'), '/link', '/Brepro', 'version.lib'], env=env, cwd=output, check=True)
    if not args.build_only:
        subprocess.run([str(output/'runtime_check.exe'), str(root/'bin/telemetry-live.dll')],check=True,timeout=30)
        missing = subprocess.run([str(output/'runtime_check.exe'),str(root/'bin/telemetry-live.dll'),
                                  'telemetry-intentionally-missing-runtime.dll'],capture_output=True,text=True,timeout=30)
        assert missing.returncode == 2 and 'Install or repair' in missing.stderr, missing
    subprocess.run(common + ['/LD', str(root/'native/core_mock.cpp'), '/Fo'+str(output/'core_mock.obj'),
                            '/link', '/OUT:'+str(output/'swofty_core.dll'), '/IMPLIB:'+str(output/'core_mock.lib')], env=env, cwd=output, check=True)
    subprocess.run(common + [str(root/'native/publisher_test.cpp'), '/Fo'+str(output/'publisher_test.obj'),
                            '/Fe'+str(output/'publisher_test.exe')], env=env, cwd=output, check=True)
    if not args.build_only:
        subprocess.run([str(output/'publisher_test.exe'),str(output/'swofty_core.dll'),str(root/'bin/telemetry-live.dll')], check=True, timeout=30)
        subprocess.run([str(output/'publisher_test.exe'),str(output/'swofty_core.dll'),str(root/'bin/telemetry-live.dll'),'verified'], check=True, timeout=30)
        subprocess.run([str(output/'publisher_test.exe'),str(output/'swofty_core.dll'),str(root/'bin/telemetry-live.dll'),'current'], check=True, timeout=30)
print('PASS: native CI artifacts compiled; tests run separately' if args.build_only else 'PASS: native readers, session lifecycle and wire ABI; no game launched')
