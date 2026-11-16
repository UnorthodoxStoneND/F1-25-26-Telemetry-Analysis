"""End-to-end recording/reload/chart checks through the real Windows helper.

Uses generated UDP only. --keep retains an isolated fixture for visual inspection.
"""
import argparse
import json
import math
import os
import pathlib
import queue
import socket
import struct
import subprocess
import tempfile
import threading
import time

ROOT = pathlib.Path(__file__).resolve().parents[1]


def validate_view(view):
    for block in view['body']['blocks']:
        assert len(block) == 1, block
        if 'table' in block:
            table = block['table']
            for row in table['rows']:
                cells = row['cells'] if isinstance(row, dict) else row
                assert len(cells) == len(table['columns'])
                assert all(isinstance(c, (str, dict)) for c in cells), cells
        if 'text' in block:
            assert block['text']['tone'] == 'neutral'
        if 'image' in block:
            assert pathlib.Path(block['image']['path']).is_absolute()
        if 'form' in block:
            for field in block['form']['fields']:
                assert len(field) == 1
                if 'select' in field:
                    f = field['select']
                    assert f['value'] in {o['value'] for o in f['options']}

def validate_image_roots(value, roots):
    if isinstance(value, dict):
        if isinstance(value.get('image'), dict):
            path = pathlib.Path(value['image']['path']).resolve()
            assert any(path.is_relative_to(root.resolve()) for root in roots), ('Swofty rejects image path', path)
            assert path.is_file(), path
        for child in value.values():
            validate_image_roots(child, roots)
    elif isinstance(value, list):
        for child in value:
            validate_image_roots(child, roots)


class Helper:
    def __init__(self, data):
        self.image_roots = (data, data.parent.parent/'cache'/'unorthodoxstonendmodding.udp-telemetry')
        self.native_views = data.parent.parent/'native-views.jsonl'
        self.proc = subprocess.Popen([str(ROOT/'bin/udp-telemetry.exe')],
            env={**os.environ, 'F1_TELEMETRY_TEST_DOCUMENTS': str(data.parent.parent/'Documents')},
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, encoding='utf-8', creationflags=subprocess.CREATE_NO_WINDOW)
        self.messages = queue.Queue()
        self.ident = 0
        def read():
            for line in self.proc.stdout:
                self.messages.put(json.loads(line))
        threading.Thread(target=read, daemon=True).start()
        assert self.call('hello', {'api': 1, 'data_dir': str(data),
            'cache_dir': str(self.image_roots[1]), 'launcher': '1.9.55', 'capabilities': ['view.chart'],
            'profile': {'name': 'fixture', 'enabled': True, 'vanilla': False}}) == {'api': 1}

    def send(self, value):
        self.proc.stdin.write(json.dumps(value)+'\n')
        self.proc.stdin.flush()

    def call(self, method, params=None):
        self.ident += 1
        self.send({'id': self.ident, 'method': method, 'params': params})
        deadline = time.monotonic()+10
        while time.monotonic() < deadline:
            message = self.messages.get(timeout=10)
            if message.get('method') == 'game_status':
                self.send({'id': message['id'], 'result': {'running': True}})
            elif message.get('id') == self.ident:
                assert 'error' not in message, message
                if method == 'view':
                    validate_image_roots(message['result'], self.image_roots)
                    with self.native_views.open('a', encoding='utf-8') as output:
                        output.write(json.dumps(message['result'])+'\n')
                return message['result']
        raise AssertionError('helper timeout')

    def view(self):
        for _ in range(60):
            view = self.call('view', {'section': 'analysis'})
            validate_view(view)
            if 'Loading saved session' not in json.dumps(view):
                return view
            time.sleep(.05)
        raise AssertionError('analysis load timeout')

    def event(self, form, values):
        return self.call('event', {'event': {'submit': {'form': form, 'values': values}}})

    def close(self):
        self.proc.stdin.close()
        assert self.proc.wait(timeout=10) == 0, self.proc.stderr.read()


def packet(fmt, kind, frame, uid, clock=0):
    slots = 22 if fmt == 2025 else 24
    sizes = {1: 753 if fmt == 2025 else 926, 2: 31+slots*57, 3: 45,
        4: 30+slots*(57 if fmt == 2025 else 60),
        6: 32+slots*(60 if fmt == 2025 else 59),
        7: 29+slots*(55 if fmt == 2025 else 59), 8: 30+slots*46,
        10: 29+slots*46, 11: 1460, 15: 31+50*slots}
    data = bytearray(sizes[kind])
    struct.pack_into('<H', data, 0, fmt)
    data[5:7] = bytes([1, kind])
    struct.pack_into('<QfII', data, 7, uid, clock, frame, frame)
    return data


def fixture(udp, port, fmt, session_type=15):
    uid = fmt*100+session_type
    def send(data):
        udp.sendto(data, ('127.0.0.1', port))
    session = packet(fmt, 1, 0, uid)
    session[29:33] = bytes([3, 45, 24, 2])
    session[35:37] = bytes([session_type, 7])
    struct.pack_into('<III', session, 670, fmt, 700, 1)
    send(session)
    names = packet(fmt, 4, 0, uid)
    names[29] = 2
    stride, off = (57, 7) if fmt == 2025 else (60, 10)
    for driver, name in enumerate((b'Fixture Driver A', b'Fixture Driver B')):
        names[30+driver*stride] = 1
        names[30+driver*stride+off:30+driver*stride+off+len(name)] = name
        start = 30+driver*stride
        if fmt == 2025:
            names[start+1] = driver
            names[start+3:start+6] = bytes([0 if driver == 0 else 255, driver, 11+driver])
        else:
            struct.pack_into('<H', names, start+1, driver)
            struct.pack_into('<H', names, start+5, 0 if driver == 0 else 65535)
            names[start+7:start+9] = bytes([driver, 11+driver])
    send(names)
    event = packet(fmt, 3, 1, uid)
    event[29:33] = b'SSTA'
    send(event)
    event[29:34] = b'STLG\x01'
    send(event)
    event[29:33] = b'LGOT'
    send(event)
    for frame in range(1, 1201):
        lap = 1+(frame-1)//600
        part = (frame-1)%600/600
        clock = frame*.15
        if frame % 30 == 1:
            session = packet(fmt, 1, frame, uid, clock)
            session[29:33] = bytes([3, 45, 24, 2])
            session[35:37] = bytes([session_type, 7])
            session[153] = 1 if 60 <= frame < 120 else (2 if 750 <= frame < 800 else 0)
            struct.pack_into('<III', session, 670, fmt, 700, 1)
            send(session)
            status = packet(fmt, 7, frame, uid, clock)
            stride = 55 if fmt == 2025 else 59
            for driver in range(2):
                status[29+driver*stride+25:29+driver*stride+28] = bytes([17 if frame < 660 else 16, 16, lap])
            send(status)
            damage = packet(fmt, 10, frame, uid, clock)
            for driver in range(2):
                struct.pack_into('<4f', damage, 29+driver*46, *([part*30]*4))
            send(damage)
        lap_data = packet(fmt, 2, frame, uid, clock)
        telemetry = packet(fmt, 6, frame, uid, clock)
        stride = 60 if fmt == 2025 else 59
        for driver in range(2):
            off = 29+driver*57
            struct.pack_into('<II', lap_data, off, 90000 if lap > 1 else 0, int(part*90000))
            struct.pack_into('<f', lap_data, off+20, part*4500)
            lap_data[off+32:off+34] = bytes([driver+1, lap])
            struct.pack_into('<H', lap_data, off+17, 0 if driver == 0 else 1200+int(part*500))
            lap_data[off+43] = driver+1
            lap_data[off+44] = 1
            if driver == 0 and 620 <= frame <= 660:
                lap_data[off+46] = 1
                struct.pack_into('<HH', lap_data, off+47, (frame-619)*600, min((frame-619)*100, 2500))
            off = 29+driver*stride
            speed = 230+75*math.sin(part*math.pi*12+.1*driver)
            throttle = max(0, math.sin(part*math.pi*12+.1*driver))
            brake = max(0, -math.sin(part*math.pi*12+.1*driver))*.7
            struct.pack_into('<HfffBbH', telemetry, off, int(speed), throttle, 0, brake, 0, 5, int(10000+speed*10))
        # Reverse some datagrams to exercise pairing.
        for data in ((telemetry, lap_data) if frame % 7 == 0 else (lap_data, telemetry)):
            send(data)
        time.sleep(.0007)
    for driver in range(2):
        history = packet(fmt, 11, 1300+driver, uid, 180)
        history[29:32] = bytes([driver, 4, 2])
        for lap in range(2):
            off = 36+lap*14
            struct.pack_into('<IHBHBHBB', history, off, 90000+driver*1000, 30000, 0, 30000, 0, 30000+driver*1000, 0, 15)
        if session_type >= 15 and driver == 1:
            history[63] = 0  # completed race lap 2 is invalid but remains included
        struct.pack_into('<I', history, 64, 80000)  # invalid faster lap 3
        history[91] = 15  # valid current partial lap 4, full time remains zero
        history[1436:1442] = bytes([1, 17, 16, 2, 16, 16])
        send(history)
    positions = packet(fmt, 15, 1400, uid, 180)
    positions[29:31] = bytes([2, 0])
    positions[31:33] = bytes([1, 2])
    slots = 22 if fmt == 2025 else 24
    positions[31+slots:33+slots] = bytes([1, 2])
    send(positions)
    event = packet(fmt, 3, 1401, uid, 180)
    event[29:35] = b'OVTK\x00\x01'
    send(event)
    results = packet(fmt, 8, 1402, uid, 180)
    results[29] = 2
    for driver in range(2):
        off = 30+driver*46
        results[off:off+7] = bytes([driver+1, 2, driver+1, 25, 1, 3, 2])
        struct.pack_into('<Id', results, off+7, 90000+driver*1000, 180+driver*2)
        results[off+21] = 2
        results[off+22:off+24] = bytes([17, 16])
        results[off+38:off+40] = bytes([1, 255])
    send(results)
    # Results must not cut off ongoing or reordered per-car telemetry.
    def late_trace(frame, distance):
        late_lap = packet(fmt, 2, frame, uid, 180+(frame-1402)*.05)
        late_car = packet(fmt, 6, frame, uid, 180+(frame-1402)*.05)
        late_lap[61:63] = bytes([2, 2])  # deliberately conflicts with final P1
        late_lap[73] = 1
        struct.pack_into('<I', late_lap, 33, 90000)  # current lap clock
        struct.pack_into('<f', late_lap, 49, distance)
        struct.pack_into('<HfffBbH', late_car, 29, 210, .5, 0, 0, 0, 5, 11000)
        send(late_car)
        send(late_lap)
    late_trace(1403, 4495)
    event[29:33] = b'SEND'
    send(event)
    late_trace(1404, 4498)
    time.sleep(.3)


def nodes(value, key):
    if isinstance(value, dict):
        if isinstance(value.get(key), dict):
            yield value[key]
        for child in value.values():
            yield from nodes(child, key)
    elif isinstance(value, list):
        for child in value:
            yield from nodes(child, key)


def all_dicts(value):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from all_dicts(child)
    elif isinstance(value, list):
        for child in value:
            yield from all_dicts(child)


def table_id(view, ident):
    return next(t for t in nodes(view, 'table') if t.get('id') == ident)


def click(helper, ident):
    return helper.call('event', {'event': {'click': {'id': ident}}})


def tab(helper, ident):
    helper.call('event', {'event': {'tab': {'id': 'analysis-tabs', 'tab': ident}}})
    return helper.view()


def select_record(helper, meta):
    helper.event('analysis-filters', {'save': meta['save']})
    helper.view()
    helper.event('analysis-filters', {'session': meta['id']})
    return helper.view()


def exercise(folder):
    folder.mkdir(parents=True, exist_ok=True)
    legacy = folder/'addon-data'/'unorthodoxstonendmodding.udp-telemetry'
    legacy.mkdir(parents=True, exist_ok=True)
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as probe:
        probe.bind(('127.0.0.1', 0))
        port = probe.getsockname()[1]
    (legacy/'settings.json').write_text(json.dumps({'mode': 'full', 'port': port}))
    helper = Helper(legacy)
    data = folder/'Documents'/'F1 Game Telemetry'
    assert (data/'settings.json').exists(), 'settings were not migrated to Documents'
    race_ids = {}
    try:
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as udp:
            for fmt in (2025, 2026):
                fixture(udp, port, fmt)
                metas = [json.loads(p.read_text()) for p in (data/'recordings').glob('*.json')]
                meta = next(m for m in metas if m['format'] == fmt and m['Session'] == 15)
                race_ids[fmt] = meta['id']
                view = select_record(helper, meta)
                fresh = json.loads((data/'recordings'/(meta['id']+'.json')).read_text())
                assert fresh['telemetry_packets_after_classification'] == 2 and fresh['telemetry_packets_after_session_end'] == 1, fresh
                assert fresh['race_capture_trigger'] == 'start_lights' and fresh['race_start_lights'] == 1
                tabs = next(nodes(view, 'tabs'))
                assert [i['label'] for i in tabs['items']] == ['Overview', 'Lap Analysis', 'Pit Stops', 'Race Strategy', 'Race Progression'], tabs
                assert tabs['active'] == 'stats'
                assert not any(b.get('id', '').startswith('analysis-race-') for b in nodes(view, 'form'))
                results_before = table_id(view, 'analysis-results')['rows']
                assert any(len(b.get('stats', [])) == 6 for b in view['body']['blocks'])
                view = tab(helper, 'traces')
                assert len(list(nodes(view, 'chart'))) == 5, view
                times = table_id(view, 'race-lap-times')
                assert len(times['rows']) == 1 and times['rows'][0][0] == 'Fixture Driver A'
                assert any(g['columns'] == [3, 2] for g in nodes(view, 'grid'))
                chip_prefix = 'analysis-race-driver-'+meta['id']+'-'
                chips = [n for n in all_dicts(view) if n.get('id', '').startswith(chip_prefix)]
                assert len(chips) == 2 and [c['style'] for c in chips] == ['primary', 'outline'], chips
                click(helper, chip_prefix+'1')
                assert len(table_id(helper.view(), 'race-lap-times')['rows']) == 2
                click(helper, chip_prefix+'1')
                assert len(table_id(helper.view(), 'race-lap-times')['rows']) == 1
                click(helper, 'analysis-race-all')
                view = helper.view()
                assert len(table_id(view, 'race-lap-times')['rows']) == 2
                helper.event('analysis-race-lap-'+meta['id'], {'lap': '2'})
                lap_rows = table_id(helper.view(), 'race-lap-times')['rows']
                assert len(lap_rows) == 2 and all(len(row) == 5 for row in lap_rows), lap_rows
                for target, count in (('tyres', 3), ('positions', 2), ('progression', 4)):
                    view = tab(helper, target)
                    assert len(list(nodes(view, 'chart'))) == count, target
                    chips = [n for n in all_dicts(view) if n.get('id', '').startswith(chip_prefix)]
                    assert len(chips) == 2 and all(c['style'] == 'primary' for c in chips), (target, chips)
                    (folder/f'{target}-{fmt}.json').write_text(json.dumps(view, indent=2))
                view = tab(helper, 'stats')
                assert table_id(view, 'analysis-results')['rows'] == results_before
                assert not any(f.get('id', '').startswith('analysis-race-') for f in nodes(view, 'form'))
                tab(helper, 'traces')
                click(helper, 'analysis-race-none')
                assert 'No drivers selected' in json.dumps(helper.view())
                click(helper, chip_prefix+'0')
                assert len(table_id(helper.view(), 'race-lap-times')['rows']) == 1
                view = tab(helper, 'laps')
                assert next(nodes(view, 'tabs'))['active'] == 'stats', 'removed race tab remained accessible'
                click(helper, 'analysis-edit-save')
                assert any(f.get('id') == 'analysis-save' for f in nodes(helper.view(), 'form'))
                helper.event('analysis-save', {'name': f'My Team {fmt}'})
                assert not any(f.get('id') == 'analysis-save' for f in nodes(helper.view(), 'form'))
                assert '0.4.2' in helper.view()['subtitle']
                live = helper.call('view', {'section': 'telemetry'})
                assert 'analysis-' not in json.dumps(live) and 'recording' not in json.dumps(live).lower()
                print(f'PASS: {fmt} race tabs, five separate traces, timing side panel, shared multi-driver selection and all strategy/progression charts')
            # Practice/quali retain their native results + valid lap comparison.
            for fmt, session_type in ((2025, 1), (2026, 1), (2026, 8)):
                fixture(udp, port, fmt, session_type)
                metas = [json.loads(p.read_text()) for p in (data/'recordings').glob('*.json')]
                meta = next(m for m in metas if m['format'] == fmt and m['Session'] == session_type)
                view = select_record(helper, meta)
                labels = [i['label'] for i in next(nodes(view, 'tabs'))['items']]
                if session_type == 1:
                    assert labels == ['Overview', 'Session Analysis'], labels
                    assert [c['title'] for c in table_id(view, 'analysis-results')['columns']] == ['POS','Full Driver Name','Team','Compounds','Laps','Fastest Lap','Gap','Avg Time']
                    view = tab(helper, 'practice')
                    assert len(list(nodes(view, 'chart'))) == 5, view
                    click(helper, 'analysis-race-all')
                    view = helper.view()
                    assert len(list(nodes(view, 'chart'))) == 5
                    chips = [n for n in all_dicts(view) if n.get('id', '').startswith('analysis-race-driver-')]
                    assert len(chips) == 2 and all(c['style'] == 'primary' for c in chips)
                    click(helper, 'analysis-race-none')
                    assert 'No drivers selected' in json.dumps(helper.view())
                    view = tab(helper, 'laps')
                    assert next(nodes(view, 'tabs'))['active'] == 'stats'
                    continue
                assert labels == ['Qualifying Results', 'Advanced Telemetry'], labels
                assert [c['title'] for c in table_id(view, 'analysis-results')['columns']] == ['POS','Full Driver Name','Team','Compounds','Laps','Fastest Lap','Gap','Avg Time']
                view = tab(helper, 'advanced')
                assert len(list(nodes(view, 'chart'))) == 2, view
                view = tab(helper, 'laps')
                assert next(nodes(view, 'tabs'))['active'] == 'stats'
            # Full qualifying and sprint qualifying link only their own stages.
            for fmt, kinds in ((2025, (5,6,7)), (2026, (10,11,12))):
                for kind in kinds:
                    fixture(udp, port, fmt, kind)
                metas = [json.loads(p.read_text()) for p in (data/'recordings').glob('*.json')]
                meta = next(m for m in metas if m['format']==fmt and m['Session']==kinds[-1])
                view = select_record(helper, meta)
                assert len([c for c in nodes(view, 'card') if c.get('title') in ['Q1','Q2','Q3','SQ1','SQ2','SQ3']]) == 3, view
                view = tab(helper, 'advanced')
                assert len(list(nodes(view, 'chart'))) == 3, view
                selector = next(f for f in nodes(view,'form') if f['id'].startswith('analysis-qual-stage-'))
                assert [int(o['value']) for o in selector['fields'][0]['select']['options']] == list(kinds), selector
                helper.event(selector['id'], {'stage': str(kinds[0])})
                view = helper.view()
                selector = next(f for f in nodes(view,'form') if f['id'].startswith('analysis-qual-stage-'))
                assert selector['fields'][0]['select']['value'] == str(kinds[0]), selector
            print('PASS: practice charts retained; short qualifying tables; linked Q1/Q2/Q3 and SQ1/SQ2/SQ3 results, Advanced Telemetry and stage selection')
    finally:
        helper.close()
    # Deterministic saved-memory fixture verifies old formatted names still display.
    for path in (data/'recordings').glob('*.json'):
        meta = json.loads(path.read_text())
        meta['team_snapshot'] = {'captured': '2026-11-14T00:00:00Z',
            'process_started': '2026-11-14T00:00:00Z', 'pid': 1,
            'mode': 'fixture', 'language': 'eng', 'exe_sha256': 'fixture',
            'teams': [{'set': 0, 'id': 0, 'label': 'fixture_constructor',
                       'raw_name': 'Fixture Custom Constructor', 'name': '{o:upper}F{/o}ixture Custom Constructor'},
                      {'set': 1, 'id': 0, 'label': 'fixture_myteam',
                       'raw_name': 'Fixture My Team', 'name': 'Fixture My Team'}]}
        path.write_text(json.dumps(meta))
    helper = Helper(legacy)
    try:
        for fmt in (2025, 2026):
            view = select_record(helper, {'save': f'My Team {fmt}', 'id': race_ids[fmt]})
            results = table_id(view, 'analysis-results')
            assert results['columns'][2]['title'] == 'Team'
            assert [r[2] for r in results['rows'][:2]] == ['Fixture Custom Constructor', 'Fixture My Team'], results
            view = tab(helper, 'traces')
            assert len(list(nodes(view, 'chart'))) == 5
        print('PASS: helper restart retains saved sessions, custom teams, association, native race charts and native medals and tyre cells')
    finally:
        helper.close()
    documents = folder/'Documents'/'F1 Game Telemetry'
    assert not list(documents.rglob('*.png')), 'Analysis generated a PNG in Documents'
    assert not list(documents.rglob('*.svg')), 'Analysis generated an SVG in Documents'
    print('PASS: no chart image files created in Documents across selections and helper restarts')
    print(f'Fixture and native chart specifications: {folder}')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--keep', type=pathlib.Path)
    args = parser.parse_args()
    if args.keep:
        exercise(pathlib.Path(tempfile.mkdtemp(prefix='analysis-', dir=args.keep)))
    else:
        with tempfile.TemporaryDirectory(prefix='swofty-analysis-test-') as directory:
            exercise(pathlib.Path(directory))
