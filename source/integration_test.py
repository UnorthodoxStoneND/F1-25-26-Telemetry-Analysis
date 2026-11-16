"""Exercise the shipped Windows helper over real pipes and loopback UDP."""
import json
import os
import pathlib
import queue
import socket
import struct
import subprocess
import tempfile
import threading
import time
from analysis_integration_test import validate_image_roots

ROOT = pathlib.Path(__file__).resolve().parents[1]


def packet(fmt, kind, frame=1, uid=123):
    n = 22 if fmt == 2025 else 24
    sizes = {1: 753 if fmt == 2025 else 926, 2: 31+n*57, 3: 45,
             4: 30+n*(57 if fmt == 2025 else 60), 5: 33+n*50,
             7: 29+n*(55 if fmt == 2025 else 59), 10: 29+n*46, 11: 1460}
    b = bytearray(sizes[kind])
    struct.pack_into('<H', b, 0, fmt)
    b[5:7] = bytes([1, kind])
    struct.pack_into('<Q', b, 7, uid)
    struct.pack_into('<I', b, 23, frame)
    return b


def table(view):
    return next((b['table'] for b in view['body']['blocks'] if 'table' in b), None)


def check_view(view, roots):
    validate_image_roots(view, roots)
    def check_tones(value):
        if isinstance(value, dict):
            if isinstance(value.get('text'), dict):
                assert value['text'].get('tone') in {'neutral', 'muted', 'accent', 'good', 'warn', 'bad'}, value
            for child in value.values():
                check_tones(child)
        elif isinstance(value, list):
            for child in value:
                check_tones(child)
    check_tones(view)
    assert set(view) <= {'subtitle', 'tick', 'body'}
    assert 'changelog' not in json.dumps(view).lower()
    for b in view['body']['blocks']:
        assert len(b) == 1
        assert next(iter(b)) in {'hero', 'form', 'facts', 'text', 'table', 'buttons', 'card'}
        if 'table' in b:
            t = b['table']
            for row in t['rows']:
                assert len(row['cells']) == len(t['columns']) == 11
                for cell in row['cells']:
                    if isinstance(cell, dict) and 'image' in cell:
                        assert pathlib.Path(cell['image']['path']).is_file()


def main():
    with tempfile.TemporaryDirectory(prefix='udp-telemetry-test-') as td:
        # Use a private random port, leaving the user's usual 20777 untouched.
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as probe:
            probe.bind(('127.0.0.1', 0))
            port = probe.getsockname()[1]
        data_dir = pathlib.Path(td, 'addon-data', 'unorthodoxstonendmodding.udp-telemetry')
        data_dir.mkdir(parents=True)
        cache_dir = pathlib.Path(td, 'cache', 'unorthodoxstonendmodding.udp-telemetry')
        (data_dir/'settings.json').write_text(json.dumps({'port': port, 'mode': 'nearest'}))
        profile_dir = pathlib.Path(td, 'profiles')
        profile_dir.mkdir()
        options = {}
        def change_options(**values):
            options.update(values)
            (profile_dir/'fixture.json').write_text(json.dumps({'options': {'unorthodoxstonendmodding.udp-telemetry': options}}))
        change_options()
        proc = subprocess.Popen([str(ROOT/'bin/udp-telemetry.exe')], stdin=subprocess.PIPE,
                                env={**os.environ, 'F1_TELEMETRY_TEST_DOCUMENTS': str(pathlib.Path(td, 'Documents'))},
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                                encoding='utf-8', creationflags=getattr(subprocess, 'CREATE_NO_WINDOW', 0))
        messages = queue.Queue()
        def read():
            for line in proc.stdout:
                messages.put(json.loads(line))
        threading.Thread(target=read, daemon=True).start()
        running = False  # Pre-grid packets must not depend on launcher startup detection.
        req_id = 0
        def send(v):
            proc.stdin.write(json.dumps(v)+'\n')
            proc.stdin.flush()
        def call(method, params=None):
            nonlocal req_id
            req_id += 1
            send({'id': req_id, 'method': method, 'params': params})
            while True:
                msg = messages.get(timeout=5)
                if msg.get('method') == 'game_status':
                    send({'id': msg['id'], 'result': {'running': running}})
                elif 'method' in msg:
                    assert msg['method'] == 'invalidate'
                elif msg.get('id') == req_id:
                    assert 'error' not in msg, msg
                    return msg['result']
        try:
            assert call('hello', {'api': 1, 'data_dir': str(data_dir), 'cache_dir': str(cache_dir),
                                 'profile': {'name': 'fixture', 'enabled': True, 'vanilla': False},
                                 'launcher': '1.9.55', 'capabilities': ['view.chart']}) == {'api': 1}
            check_view(call('view', {'section': 'telemetry'}), (data_dir, cache_dir))
            with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as udp:
                def transmit(b):
                    udp.sendto(b, ('127.0.0.1', port))
                for fmt in (2025, 2026):
                    n = 22 if fmt == 2025 else 24
                    uid = fmt
                    participants = packet(fmt, 4, uid=uid)
                    participants[29] = n
                    stride, name, my = (57, 7, 4) if fmt == 2025 else (60, 10, 7)
                    for i in range(n):
                        off = 30+i*stride
                        participants[off] = 1
                        participants[off+my] = 1
                        label = f'Driver {i+1}'.encode()
                        participants[off+name:off+name+len(label)] = label
                    transmit(participants)
                    lap = packet(fmt, 2, frame=2, uid=uid)
                    for i in range(n):
                        off = 29+i*57
                        struct.pack_into('<I', lap, off, 91234)
                        lap[off+32], lap[off+33], lap[off+45] = i+1, 3, 2
                    transmit(lap)
                    damage = packet(fmt, 10, frame=3, uid=uid)
                    for i in range(n):
                        struct.pack_into('<4f', damage, 29+i*46, 10, 20, 30, 40)
                    transmit(damage)
                    status = packet(fmt, 7, frame=4, uid=uid)
                    stride = 55 if fmt == 2025 else 59
                    for i in range(n):
                        status[29+i*stride+25:29+i*stride+28] = bytes([18, 16, 2])
                    transmit(status)
                    setups = packet(fmt, 5, frame=5, uid=uid)
                    for i in range(n):
                        off = 29+i*50
                        setups[off:off+4] = bytes([20, 25, 80, 30])
                        struct.pack_into('<f', setups, off+37, 24.5)
                    transmit(setups)
                    hist = packet(fmt, 11, frame=6, uid=uid)
                    hist[30] = 2
                    struct.pack_into('<I', hist, 36, 90000)
                    hist[49] = 1
                    struct.pack_into('<I', hist, 50, 80000)  # invalid faster lap
                    transmit(hist)
                    time.sleep(.15)
                    view = call('view', {'section': 'telemetry'})
                    check_view(view, (data_dir, cache_dir))
                    rows = table(view)['rows']
                    assert len(rows) == 5, {'format': fmt, 'rows': len(rows), 'view': view}
                    tyre = rows[0]['cells'][2]
                    assert tyre['text'] == 'C3' and 'image' not in tyre
                    assert tyre['edit']['colour'] == {'value': '#FF4A5B', 'disabled': True}
                    assert [c['text'] if isinstance(c, dict) else c for c in rows[0]['cells'][3:10]] == ['30.0%', '40.0%', '10.0%', '20.0%', '25.0%', '1:31.234', '1:30.000']
                    assert rows[0]['cells'][7]['bar'] == .25
                    assert rows[0]['cells'][1] == 'Driver 1'
                    assert 'tone' not in rows[0] and 'detail' not in rows[0]
                    change_options(highlight_player=True)
                    assert table(call('view', {'section': 'telemetry'}))['rows'][0]['tone'] == 'accent'
                    change_options(highlight_player=False)
                    call('event', {'event': {'submit': {'form': 'drivers', 'row': 'car-1', 'values': {'setup': True}}}})
                    expanded = call('view', {'section': 'telemetry'})
                    blocks = expanded['body']['blocks']
                    card_index = next(k for k, block in enumerate(blocks) if 'card' in block)
                    assert blocks[card_index-1]['table']['rows'][-1]['id'] == 'car-1'
                    assert blocks[card_index+1]['table']['rows'][0]['id'] == 'car-2'
                    assert len(blocks[card_index]['card']['blocks'][0]['table']['rows']) == 11
                    call('event', {'event': {'submit': {'form': 'drivers', 'row': 'car-1', 'values': {'setup': False}}}})
                    change_options(drivers_shown='full')
                    assert len(table(call('view', {'section': 'telemetry'}))['rows']) == n
                    # Player in last position: the window shifts to four ahead.
                    lap[27] = n-1
                    struct.pack_into('<I', lap, 23, 7)
                    transmit(lap)
                    change_options(drivers_shown='nearest')
                    time.sleep(.05)
                    rows = table(call('view', {'section': 'telemetry'}))['rows']
                    assert [r['cells'][0] for r in rows] == [str(k) for k in range(n-4, n+1)]
                    paused = packet(fmt, 1, frame=8, uid=uid)
                    paused[35], paused[36], paused[43] = 15, 4, 1
                    paused[27] = n-1
                    transmit(paused)
                    time.sleep(.05)
                    running = True
                    assert len(table(call('view', {'section': 'telemetry'}))['rows']) == 5
                    change_options(drivers_shown='none')
                    assert table(call('view', {'section': 'telemetry'})) is None
                    change_options(drivers_shown='nearest')
                    # Exit from a paused session can share an event frame or
                    # reset lifecycle counters; normal sample ordering must
                    # never suppress that session-end event.
                    button = packet(fmt, 3, frame=9, uid=uid)
                    button[29:33] = b'BUTN'
                    transmit(button)
                    ended = packet(fmt, 3, frame=0, uid=0)
                    ended[29:33] = b'SEND'
                    transmit(ended)
                    transmit(damage)  # a late datagram cannot revive the session
                    time.sleep(.05)
                    assert table(call('view', {'section': 'telemetry'}))['rows'] == []
                    print(f'PASS: UDP {fmt}, {n} cars, My Team, setup, wear, timing, nearest/full/none, pause/end')
                # Establish a running game then confirm its exit.
                running = True
                call('view', {'section': 'telemetry'})
                p = packet(2026, 4, uid=9999)
                p[29] = 1
                p[40:44] = b'Test'
                transmit(p)
                time.sleep(.05)
                running = False
                stopped = call('view', {'section': 'telemetry'})
                assert table(stopped)['rows'] == []
                assert 'Game stopped' in json.dumps(stopped)
                call('tick', {'section': 'telemetry'})
                print('PASS: launcher status, tick/invalidate, native view shape, no changelog UI')
        finally:
            proc.stdin.close()
            proc.wait(timeout=5)
            assert proc.returncode == 0, proc.stderr.read()


if __name__ == '__main__':
    main()
