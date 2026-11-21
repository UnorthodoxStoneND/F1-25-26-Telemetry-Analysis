package main

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Exercise the real background protocol/reader loop with our synthetic IPC,
// without a view request, OS window, F1 process or game-memory access.
func TestDesktopBackgroundStartupAndShutdown(t *testing.T) {
	dir, path, bytes := linuxBridge(t)
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	put64 := func(at int, value uint64) {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, value)
		if _, err := file.WriteAt(buf, int64(at)); err != nil {
			t.Fatal(err)
		}
	}
	put64(32, 100)
	binary.LittleEndian.PutUint32(bytes[40:], binary.LittleEndian.Uint32(bytes[40:])|windowReady)
	if _, err := file.WriteAt(bytes[40:44], 40); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	data := filepath.Join(root, "addon-data", addonID)
	if err := os.MkdirAll(filepath.Join(root, "profiles"), 0700); err != nil {
		t.Fatal(err)
	}
	options, _ := json.Marshal(M{"options": M{addonID: M{"display_mode": "window", "drivers_shown": "full"}}})
	if err := os.WriteFile(filepath.Join(root, "profiles", "main.json"), options, 0600); err != nil {
		t.Fatal(err)
	}
	in, writer := io.Pipe()
	defer in.Close()
	defer writer.Close()
	bridge := newDesktopBridge()
	done := make(chan error, 1)
	go func() { done <- runWithDesktop(in, io.Discard, bridge) }()
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(M{"id": 1, "method": "hello", "params": M{"api": 1, "data_dir": data, "profile": M{"name": "main", "enabled": true}}}); err != nil {
		t.Fatal(err)
	}
	encoder.Encode(M{"id": 2, "method": "event", "params": M{"section": "telemetry", "event": M{"startup": M{}}}})
	encoder.Encode(M{"id": 1000001, "result": dir})
	await := func(check func(retroModel) bool) retroModel {
		t.Helper()
		deadline := time.NewTimer(4 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case model := <-bridge.models:
				if check(model) {
					return model
				}
			case <-deadline.C:
				t.Fatal("background desktop transition timed out")
				return retroModel{}
			}
		}
	}
	await(func(m retroModel) bool { return len(m.Rows) == 1 })
	put64(32, 101)
	shown := await(func(m retroModel) bool { return m.Show })
	if shown.Rows[0].Cells[1].PNG == "" {
		t.Fatal("background snapshot lost team image")
	}
	bridge.act(desktopAction{Kind: "dismiss"})
	await(func(m retroModel) bool { return !m.Show })
	// A real end hides the window and EOF terminates the helper's owner loop.
	binary.LittleEndian.PutUint32(bytes[40:], 32)
	if _, err := file.WriteAt(bytes[40:44], 40); err != nil {
		t.Fatal(err)
	}
	await(func(m retroModel) bool { return !m.Show && len(m.Rows) == 0 })
	writer.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("helper survived EOF")
	}
}
