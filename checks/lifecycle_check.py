"""Exercise the compiled helper's UDP ownership and clean shutdown in isolation."""
import json
from pathlib import Path
import socket
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "source"))
from analysis_integration_test import Helper, packet


def port_available(port):
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as probe:
        try:
            probe.bind(("127.0.0.1", port))
        except OSError:
            return False
    return True


def main():
    with tempfile.TemporaryDirectory(prefix="telemetry-lifecycle-") as temporary:
        folder = Path(temporary)
        legacy = folder / "addon-data" / "fixture"
        legacy.mkdir(parents=True)
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as probe:
            probe.bind(("127.0.0.1", 0))
            port = probe.getsockname()[1]
        (legacy / "settings.json").write_text(json.dumps({"mode": "full", "port": port}))
        helper = Helper(legacy)
        try:
            assert not port_available(port), "Enabled helper does not own its UDP port"
            helper.call("profile", {"name": "fixture", "enabled": False, "vanilla": False})
            assert port_available(port), "Disabled profile retained its UDP listener"
            helper.call("profile", {"name": "fixture", "enabled": True, "vanilla": False})
            assert not port_available(port), "Re-enabled helper did not reopen its UDP listener"
            # Leave a practice recording open, then simulate Swofty closing its pipe.
            with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as udp:
                session = packet(2026, 1, 1, 12345)
                session[35], session[36] = 1, 4
                udp.sendto(session, ("127.0.0.1", port))
            recordings = folder / "Documents" / "F1 Game Telemetry" / "recordings"
            deadline = time.monotonic() + 5
            while time.monotonic() < deadline:
                metas = [json.loads(p.read_text()) for p in recordings.glob("*.json")]
                if any(meta["state"] == "recording" for meta in metas):
                    break
                time.sleep(.05)
            else:
                raise AssertionError("Test recording was not started")
            helper.close()
            assert port_available(port), "Exited helper retained its UDP listener"
            metas = [json.loads(p.read_text()) for p in recordings.glob("*.json")]
            assert len(metas) == 1 and metas[0]["state"] == "interrupted", "Open recording was not finalised on shutdown"
            assert list(recordings.glob("*.udp.gz")), "Recording journal missing"
        finally:
            if helper.proc.poll() is None:
                helper.proc.kill()
                helper.proc.wait(timeout=5)
    print("PASS: enable/disable releases and reopens UDP; pipe closure exits helper and finalises the recording")


if __name__ == "__main__":
    main()
