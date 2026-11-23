"""Check the Live State acquisition and package boundary."""
from pathlib import Path
import json
import re

root=Path(__file__).resolve().parents[2]
m=json.loads((root/'swofty.json').read_text(encoding='utf-8'))
assert m['kind']==['native']
assert {o['id'] for o in m['options']}=={'drivers_shown','highlight_player','show_practice','show_qualifying','show_race','display_mode','sliding_animation','enable_analysis'}
assert {c['value'] for c in m['options'][0]['values']}=={'nearest','full'}
assert not any(o.get('hidden',False) for o in m['options'])
assert [s['id'] for s in m['launcher']['sections']]==['telemetry','analysis']
assert next(o for o in m['options'] if o['id']=='enable_analysis')['default'] is False
assert m['launcher']['program']['linux']=='bin/telemetry-live-linux'
for file in (root/'source').glob('*.go'):
    if file.name.endswith('_test.go'):continue
    text=file.read_text(encoding='utf-8')
    if file.name == 'generate_icon.go':
        assert text.startswith('//go:build ignore\n'), 'Icon generator must stay excluded from runtime builds'
        continue
    assert not re.search(r'"(?:net|net/[^\"]+)"|ListenUDP|exec\.Command|OpenProcess\(0x1[fF]',text),file.name
    assert '"chart"' not in text,file.name
    if file.name not in {'preferences.go','window_preferences.go','analysis_capture.go','analysis_view.go'}: assert not re.search(r'os\.(?:WriteFile|Create|Mkdir|Rename|Remove)',text),file.name
    if file.name != 'badges.go': assert '"image"' not in text,file.name
native=(root/'native/telemetry.cpp').read_text(encoding='utf-8')
assert 'Car_Tyre_UpdateWear' not in native and 'SWOFTY_MIXIN_WRAP' not in native
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
assert 'Sess_LapTimes_' not in native and 'stint_counter' not in native
assert 'car_detail' in (root/'native/live_api.h').read_text(encoding='utf-8')
assert 'void wear(' not in (root/'native/readers.h').read_text(encoding='utf-8')
assert 'void timing(' not in (root/'native/readers.h').read_text(encoding='utf-8')
proton=(root/'native/proton_transport.h').read_text(encoding='utf-8')
assert 'FILE_ATTRIBUTE_TEMPORARY|FILE_FLAG_DELETE_ON_CLOSE' in proton and 'CREATE_NEW' in proton
linux=(root/'source/mapping_linux.go').read_text(encoding='utf-8')
assert 'syscall.PROT_READ' in linux and 'syscall.O_NOFOLLOW' in linux
print('PASS: Live State policy, opt-in CSV recording, service-backed images and temporary Proton IPC; no network receiver')
