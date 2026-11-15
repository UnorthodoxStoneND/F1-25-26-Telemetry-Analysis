package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeLauncherSettings(t *testing.T) {
	root := t.TempDir()
	a := App{data: filepath.Join(root, "addon-data", "unorthodoxstonendmodding.udp-telemetry"), profileName: "test", savedPort: 20800, prefs: Preferences{Mode: "full", Port: 20800, Highlight: true}}
	a.launcherData = a.data
	path := filepath.Join(root, "profiles", "test.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(options M) {
		b, _ := json.Marshal(M{"options": M{"unorthodoxstonendmodding.udp-telemetry": options}})
		if err := os.WriteFile(path, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(M{})
	a.syncOptions()
	if a.prefs.Mode != "nearest" || a.prefs.Highlight || a.prefs.Port != 20800 {
		t.Fatal("native defaults / saved port", a.prefs)
	}
	a.tel.Active = true
	a.tel.Cars[0] = Car{Participant: true, Name: "Keep my samples"}
	write(M{"drivers_shown": "full", "highlight_player": true, "udp_port": "saved"})
	a.syncOptions()
	if a.prefs.Mode != "full" || !a.prefs.Highlight || a.tel.Cars[0].Name != "Keep my samples" {
		t.Fatal("display setting reset telemetry")
	}
	write(M{"drivers_shown": "none", "udp_port": "20801"})
	a.syncOptions()
	if a.prefs.Mode != "none" || a.prefs.Port != 20801 {
		t.Fatal("native option refresh", a.prefs)
	}
	write(M{"udp_port": "saved"})
	a.syncOptions()
	if a.prefs.Port != 20800 {
		t.Fatal("saved port overwritten")
	}
	a.profileName = "../test"
	a.prefs.Mode = "full"
	a.syncOptions()
	if a.prefs.Mode != "full" {
		t.Fatal("invalid profile name followed")
	}
}

func TestNoTelemetrySettingsOrHelpAndOnlyEditableSetup(t *testing.T) {
	a := App{enabled: true, prefs: Preferences{Mode: "full", Port: 20777}}
	a.tel.Active = true
	a.tel.Cars[0] = Car{Participant: true, Setup: make([]byte, 50)}
	a.expandedSet = true
	b, err := json.Marshal(a.telemetryView())
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, unwanted := range []string{"Drivers shown", "Highlight my row", "UDP port", "Wear bars show", "Engine braking", "Ballast", "\"form\""} {
		if strings.Contains(s, unwanted) {
			t.Fatalf("unwanted UI %q", unwanted)
		}
	}
	for _, want := range []string{"Front wing", "Rear wing", "Front camber", "Front brake bias", "Front-left pressure", "Fuel load"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing editable setup %q", want)
		}
	}
}
