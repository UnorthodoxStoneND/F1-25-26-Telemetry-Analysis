"""Check the live-only acquisition and package boundary."""
from pathlib import Path
import json
import re

root=Path(__file__).resolve().parents[2]
m=json.loads((root/'swofty.json').read_text(encoding='utf-8'))
assert m['version']=='0.5.6' and m['kind']==['native']
assert {o['id'] for o in m['options']}=={'drivers_shown','highlight_player'}
assert {c['value'] for c in m['options'][0]['values']}=={'nearest','full'}
assert not any(o.get('hidden',False) for o in m['options'])
assert len(m['launcher']['sections'])==1
for file in (root/'source').glob('*.go'):
    if file.name.endswith('_test.go'):continue
    text=file.read_text(encoding='utf-8')
    assert not re.search(r'"(?:net|net/[^\"]+)"|ListenUDP|os\.(?:WriteFile|Create|Mkdir)|exec\.Command|Recorder|OpenProcess\(0x1[fF]',text),file.name
    assert '"image"' not in text and '"chart"' not in text,file.name
native=(root/'native/telemetry.cpp').read_text(encoding='utf-8')
assert 'Car_Tyre_UpdateWear' not in native
assert 'WriteProcessMemory' not in native and 'CreateThread' not in native and 'CreateFileW(' not in native
builds=(root/'native/builds.h').read_text(encoding='utf-8')
assert 'verified_layout()' in native and '1789118616' in builds and '1794312636' in builds
assert 'SwoftyCoreLive' in native and 'resolve_binding(' in native
assert 'api->telemetry' not in native
assert 'Sess_LapTimes_GetCarData' in native and 'Sess_LapTimes_ReportNewLapTime' in native
print('PASS: live-only manifest, launcher settings, no network receiver/recorder/images or wear-function hook')
