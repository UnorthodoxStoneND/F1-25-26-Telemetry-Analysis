"""Check the live-only acquisition and package boundary."""
from pathlib import Path
import json
import re

root=Path(__file__).resolve().parents[2]
m=json.loads((root/'swofty.json').read_text(encoding='utf-8'))
assert m['version']=='0.5.10-test.4' and m['kind']==['native']
assert {o['id'] for o in m['options']}=={'drivers_shown','highlight_player','show_practice','show_qualifying','show_race'}
assert {c['value'] for c in m['options'][0]['values']}=={'nearest','full'}
assert not any(o.get('hidden',False) for o in m['options'])
assert len(m['launcher']['sections'])==1
assert m['launcher']['program']['linux']=='bin/telemetry-live-linux'
for file in (root/'source').glob('*.go'):
    if file.name.endswith('_test.go'):continue
    text=file.read_text(encoding='utf-8')
    assert not re.search(r'"(?:net|net/[^\"]+)"|ListenUDP|exec\.Command|Recorder|OpenProcess\(0x1[fF]',text),file.name
    assert '"chart"' not in text,file.name
    if file.name != 'preferences.go': assert not re.search(r'os\.(?:WriteFile|Create|Mkdir|Rename|Remove)',text),file.name
    if file.name != 'badges.go': assert '"image"' not in text,file.name
native=(root/'native/telemetry.cpp').read_text(encoding='utf-8')
assert 'Car_Tyre_UpdateWear' not in native
assert 'WriteProcessMemory' not in native and 'CreateThread' not in native and 'CreateFileW(' not in native
builds=(root/'native/builds.h').read_text(encoding='utf-8')
assert 'verified_layout()' in native and '1789118616' in builds and '1794312636' in builds
assert 'SwoftyCoreLive' in native and 'resolve_binding(' in native
assert 'SwoftyCoreCarOps' in native
direct=(root/'native/direct_live.h').read_text(encoding='utf-8')
for removed in ('g_Sess_VehicleTable','g_Sess_VehicleCount','g_Live_ControllerSystem','pit_lane(','compound(','name_at('):
    assert removed not in native and removed not in direct, removed
assert 'driver_name' in (root/'native/live_api.h').read_text(encoding='utf-8')
assert 'api->telemetry' not in native
assert 'Sess_LapTimes_GetCarData' in native and 'Sess_LapTimes_ReportNewLapTime' in native
proton=(root/'native/proton_transport.h').read_text(encoding='utf-8')
assert 'FILE_ATTRIBUTE_TEMPORARY|FILE_FLAG_DELETE_ON_CLOSE' in proton and 'CREATE_NEW' in proton
linux=(root/'source/mapping_linux.go').read_text(encoding='utf-8')
assert 'syscall.PROT_READ' in linux and 'syscall.O_NOFOLLOW' in linux
print('PASS: live-only policy, service-backed images, UI preferences and temporary Proton IPC; no recordings or network receiver')
