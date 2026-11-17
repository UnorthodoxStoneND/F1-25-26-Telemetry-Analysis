"""Smoke-test the packaged helper protocol and EOF shutdown without a game."""
from pathlib import Path
import json
import subprocess
import tempfile

root=Path(__file__).resolve().parents[1]
with tempfile.TemporaryDirectory() as temp:
    path=Path(temp)/'unused'
    requests=[{'id':1,'method':'hello','params':{'api':1,'launcher':'1.9.55','data_dir':str(path),'profile':{'name':'default','enabled':True}}},
              {'id':2,'method':'view','params':{'section':'telemetry'}},
              {'id':3,'method':'view','params':{'section':'analysis'}},
              {'id':4,'method':'summary','params':{}}]
    result=subprocess.run([str(root/'bin/telemetry-live.exe')],input=''.join(json.dumps(r)+'\n' for r in requests),capture_output=True,text=True,timeout=10,check=True)
    replies=[json.loads(line) for line in result.stdout.splitlines()]
    assert len(replies)==4 and replies[0]['result']['api']==1 and replies[1]['result']['tick'] is True
    assert 'error' in replies[2] and 'live display only' in str(replies[3])
    assert not path.exists() and not result.stderr
print('PASS: built helper handshake, live view, removed analysis, no files, clean EOF shutdown')
