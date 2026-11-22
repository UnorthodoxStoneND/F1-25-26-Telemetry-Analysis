package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func linuxBridge(t *testing.T) (string, string, []byte) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, protonFrameSize)
	copy(b, fixture())
	put := binary.LittleEndian.PutUint32
	binary.LittleEndian.PutUint64(b[16:], 2)
	binary.LittleEndian.PutUint64(b[24:], 42)
	put(b[260:], knownName|knownPosition|knownTyre|knownTyreAge)
	put(b[556:], 3)
	put(b[264:], 1)
	put(b[272:], 18)
	binary.LittleEndian.PutUint64(b[528:], 1234)
	images := badgeFixture()
	binary.LittleEndian.PutUint64(images[16:], 2)
	at := 32 + maxCars*badgeRecordSize
	copy(images[at:at+badgeRecordSize], images[32:32+badgeRecordSize])
	copy(b[frameSize:], images)
	path := filepath.Join(bin, ".telemetry-live-1-1.ipc")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	return dir, path, b
}
func TestLinuxSharedSnapshotLifecycle(t *testing.T) {
	dir, path, b := linuxBridge(t)
	r := Reader{}
	r.setAddonDir(dir)
	defer r.Close()
	first, err := r.Read()
	if err != nil || len(first.Cars) != 1 || first.Cars[0].BadgePNG == "" || first.Cars[0].TyrePNG == "" || first.Cars[0].TyreAgeLaps != 3 {
		t.Fatal("Linux paired image snapshot failed", err)
	}
	held := first.Cars[0]
	// A newer image frame alone is never mixed into the prior table.
	binary.LittleEndian.PutUint64(b[frameSize+16:], 4)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = f.WriteAt(b, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Read(); !errors.Is(err, errUpdating) {
		t.Fatal("mixed Linux generations accepted", err)
	}
	if first.Cars[0] != held {
		t.Fatal("rejected frame mutated retained table")
	}
	// Explicit end clears immediately even with an updating image frame.
	binary.LittleEndian.PutUint64(b[16:], 4)
	binary.LittleEndian.PutUint32(b[40:], 32)
	binary.LittleEndian.PutUint64(b[frameSize+16:], 5)
	if _, err = f.WriteAt(b, 0); err != nil {
		t.Fatal(err)
	}
	ended, err := r.Read()
	if err != nil || len(ended.Cars) != 0 {
		t.Fatal("Linux end waited for images", err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Read(); err == nil || r.address != 0 {
		t.Fatal("Linux exit did not release mapping")
	}
}
func TestLinuxDiscoveryRejectsInvalidMappings(t *testing.T) {
	dir, path, _ := linuxBridge(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other")
	if err := os.WriteFile(other, make([]byte, protonFrameSize), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, path); err != nil {
		t.Fatal(err)
	}
	r := Reader{}
	r.setAddonDir(dir)
	defer r.Close()
	if _, err := r.Read(); err == nil || r.address != 0 {
		t.Fatal("symlinked mapping accepted")
	}
	r.setAddonDir("relative/path")
	if r.attach() {
		t.Fatal("relative game directory accepted")
	}
}
func TestLinuxLauncherHostDirectory(t *testing.T) {
	dir, _, _ := linuxBridge(t)
	encoded, _ := json.Marshal(dir)
	hello := `{"id":1,"method":"hello","params":{"api":1,"data_dir":"` + t.TempDir() + `","profile":{"name":"main","enabled":true}}}`
	view := `{"id":2,"method":"view","params":{"section":"telemetry"}}`
	reply := `{"id":1000001,"result":` + string(encoded) + `}`
	var output bytes.Buffer
	if err := run(strings.NewReader(hello+"\n"+view+"\n"+reply+"\n"), &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, `"method":"addon_game_dir"`) || !strings.Contains(text, "Test Driver") || strings.Contains(text, `"error"`) {
		t.Fatal("Linux host call or live view failed", text)
	}
}
