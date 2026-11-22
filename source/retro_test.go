package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWindowLifecycle(t *testing.T) {
	now := time.Unix(100, 0)
	f := grid()
	f.Tick = 100
	// A fresh, readable grid behind session selection must never open a window.
	var selection windowLifecycle
	selection.observe(true, f, now)
	f.Tick++
	if selection.observe(true, f, now.Add(time.Second)) {
		t.Fatal("selection screen opened window")
	}
	f.State |= windowReady
	var life windowLifecycle
	if life.observe(true, f, now) {
		t.Fatal("opened an unverified stale first sample")
	}
	if life.observe(true, f, now.Add(time.Second)) {
		t.Fatal("opened unchanged sample")
	}
	f.Tick++
	if !life.observe(true, f, now.Add(time.Second)) {
		t.Fatal("did not open fresh session")
	}
	f.State |= paused
	f.Tick++
	if !life.observe(true, f, now.Add(2*time.Second)) {
		t.Fatal("pause closed window")
	}
	if life.observe(true, f, now.Add(6*time.Second)) {
		t.Fatal("dead publisher left window open")
	}
	f.Tick++
	if !life.observe(true, f, now.Add(7*time.Second)) {
		t.Fatal("did not recover publisher")
	}
	life.dismiss()
	f.Tick++
	if life.observe(true, f, now.Add(8*time.Second)) {
		t.Fatal("manual close immediately reopened")
	}
	f.Session++
	f.Tick++
	if life.observe(true, f, now.Add(9*time.Second)) {
		t.Fatal("restart did not await freshness")
	}
	f.Tick++
	if !life.observe(true, f, now.Add(10*time.Second)) {
		t.Fatal("new session did not reopen")
	}
	if life.observe(false, f, now.Add(11*time.Second)) {
		t.Fatal("launcher and window active together")
	}
	life.observe(true, f, now.Add(12*time.Second))
	f.Tick++
	life.observe(true, f, now.Add(13*time.Second))
	f.State = 32
	if life.observe(true, f, now.Add(14*time.Second)) {
		t.Fatal("session end did not hide")
	}
	f.State = 0
	if life.observe(true, f, now.Add(15*time.Second)) {
		t.Fatal("hub opened window")
	}
}

func TestWindowSelectionBetweenSessions(t *testing.T) {
	now := time.Unix(100, 0)
	f := grid()
	f.State |= windowReady
	var life windowLifecycle
	life.observe(true, f, now)
	f.Tick++
	if !life.observe(true, f, now.Add(time.Second)) {
		t.Fatal("actual session did not open")
	}
	f.State &^= windowReady
	f.Tick++
	if life.observe(true, f, now.Add(2*time.Second)) {
		t.Fatal("fresh pre-session data kept window open")
	}
	f.Tick++
	if life.observe(true, f, now.Add(3*time.Second)) {
		t.Fatal("selection screen reopened window")
	}
	f.State |= windowReady
	f.Tick++
	if life.observe(true, f, now.Add(4*time.Second)) {
		t.Fatal("did not recheck freshness on entering gameplay")
	}
	f.Tick++
	if !life.observe(true, f, now.Add(5*time.Second)) {
		t.Fatal("next actual session did not open")
	}
}

func TestPostRaceRewardsHideWithLiveSnapshot(t *testing.T) {
	now := time.Unix(100, 0)
	f := grid()
	f.State |= windowReady
	f.Tick = 100
	var life windowLifecycle
	life.observe(true, f, now)
	f.Tick++
	if !life.observe(true, f, now.Add(time.Second)) {
		t.Fatal("standings did not retain the trackside window")
	}
	// The native screen observer clears only WINDOW_READY on Finances/rewards;
	// car rows, ACTIVE and fresh publisher ticks can legitimately remain.
	f.State &^= windowReady
	for n := 2; n < 6; n++ {
		f.Tick++
		if life.observe(true, f, now.Add(time.Duration(n)*time.Second)) {
			t.Fatal("live post-race data reopened the window on rewards")
		}
	}
	f.State |= windowReady
	f.Tick++
	if life.observe(true, f, now.Add(6*time.Second)) {
		t.Fatal("next track entry skipped the freshness check")
	}
	f.Tick++
	if !life.observe(true, f, now.Add(7*time.Second)) {
		t.Fatal("next genuine track entry did not reopen the window")
	}
}

func TestRetroSessionParity(t *testing.T) {
	for _, typ := range []uint32{1, 5, 10, 15} {
		a := App{frame: grid(), mode: "full", status: "Live", highlight: true}
		a.frame.SessionType = typ
		a.frame.SessionKind = 3
		for i := range a.frame.Cars {
			c := &a.frame.Cars[i]
			c.Known |= knownLast | knownBest | knownResult | knownLaps | knownTyreAge | knownGapAhead
			c.Last = 92000
			c.Best = 91000 + uint32(i)*100
			c.Laps = 5
			c.TyreAgeLaps = 3
			c.Flags = 2 << 8
			c.BadgePNG = "team"
			c.TyrePNG = "tyre"
		}
		m := a.retroModel(true)
		layout := layoutFor(a.frame)
		if len(m.Columns) != len(columns(layout)) || len(m.Rows) != 24 {
			t.Fatalf("missing session %d fields/rows", typ)
		}
		for i, row := range m.Rows {
			want := driverCells(a.frame.Cars[i], layout, sessionBest(a.frame), false)
			for j, cell := range want {
				got := row.Cells[j]
				expected := retroValue(cell)
				if got.Text != expected.Text || got.PNG != expected.PNG {
					t.Fatalf("field mismatch %d/%d/%d", typ, i, j)
				}
			}
		}
		a.hidden[layout] = fieldBit(layout, "team") | fieldBit(layout, "last")
		hidden := a.retroModel(true)
		if len(hidden.Columns) != len(m.Columns)-2 || len(hidden.Rows[0].Cells) != len(hidden.Columns) {
			t.Fatal("hidden columns not applied")
		}
		for _, col := range hidden.Columns {
			if col.Key == "team" || col.Key == "last" {
				t.Fatal("hidden field shown")
			}
		}
		a.disabled[layout] = true
		if off := a.retroModel(true); len(off.Rows) != 0 || !strings.Contains(off.Message, "Disabled") {
			t.Fatal("disabled session visible")
		}
	}
}

func TestDisplayPreferenceAndExclusion(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "addon-data", addonID)
	if err := os.MkdirAll(filepath.Join(root, "profiles"), 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(M{"options": M{addonID: M{"display_mode": "window"}}})
	if err := os.WriteFile(filepath.Join(root, "profiles", "test.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	a := App{data: data, profileName: "test", enabled: true, frame: grid()}
	a.syncOptions()
	if a.displayMode != "window" {
		t.Fatal("display preference ignored")
	}
	bytes, _ := json.Marshal(a.view())
	if strings.Contains(string(bytes), "image_data") || strings.Contains(string(bytes), `"table"`) || !strings.Contains(string(bytes), "Retro window selected") {
		t.Fatal("window mode retained launcher telemetry")
	}
	b := App{data: data, profileName: "test"}
	b.syncOptions()
	if b.displayMode != "window" {
		t.Fatal("display preference lost at restart")
	}
	a.displayMode = "launcher"
	if reflect.DeepEqual(a.view(), b.view()) {
		t.Fatal("switching mode failed to restore launcher view")
	}
}

func TestRetroActionsAndSavedGroups(t *testing.T) {
	a := App{data: t.TempDir(), profileName: "test", frame: grid(), mode: "full"}
	a.frame.SessionType = 5
	a.retroAction(desktopAction{Kind: "field", Key: "last", Layout: layoutQualifying, Visible: false})
	if a.fieldVisible(layoutQualifying, "last") || !a.fieldVisible(layoutRace, "last") {
		t.Fatal("groups not isolated")
	}
	b := App{data: a.data, profileName: "test"}
	b.loadColumns()
	if b.hidden != a.hidden {
		t.Fatal("standalone field preferences did not persist")
	}
	a.retroAction(desktopAction{Kind: "setup", Row: rowID(a.frame, a.frame.Cars[0]), Layout: layoutQualifying})
	if m := a.retroModel(true); len(m.Setup) != 21 || m.SetupTitle == "" {
		t.Fatal("setup interaction missing")
	}
	a.retroAction(desktopAction{Kind: "reset", Layout: layoutQualifying})
	if a.hidden[layoutQualifying] != 0 {
		t.Fatal("reset missing")
	}
	a.frame.SessionType = 15
	a.frame.SessionKind = 3
	a.expanded = ""
	a.retroAction(desktopAction{Kind: "setup", Row: rowID(a.frame, a.frame.Cars[0]), Layout: layoutRace})
	if a.expanded != "" {
		t.Fatal("race gained nonexisting setup field")
	}
}

func TestSlidingAnimationProfileOption(t *testing.T) {
	root := t.TempDir()
	profiles := filepath.Join(root, "profiles")
	if err := os.MkdirAll(profiles, 0700); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "addon-data", addonID)
	for _, tt := range []struct {
		value   any
		disable bool
	}{{nil, false}, {true, false}, {false, true}, {"invalid", false}} {
		options := M{}
		if tt.value != nil {
			options["sliding_animation"] = tt.value
		}
		raw, _ := json.Marshal(M{"options": M{addonID: options}})
		if err := os.WriteFile(filepath.Join(profiles, "test.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		a := App{data: data, profileName: "test", disableSlides: !tt.disable}
		a.syncOptions()
		if a.disableSlides != tt.disable || a.retroModel(true).DisableSlides != tt.disable {
			t.Fatal("sliding setting/default not applied", tt)
		}
		restarted := App{data: data, profileName: "test"}
		restarted.syncOptions()
		if restarted.disableSlides != tt.disable {
			t.Fatal("sliding setting lost after restart")
		}
	}
}
