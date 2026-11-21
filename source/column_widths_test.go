package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestColumnWidthsSavedByProfileAndView(t *testing.T) {
	a := App{data: t.TempDir(), profileName: "profile", frame: grid()}
	a.frame.SessionType = 5
	a.retroAction(desktopAction{Kind: "width", Key: "driver", Width: 240, Layout: layoutQualifying, PreferenceKey: a.preferencePath()})
	if a.widths[layoutQualifying]["driver"] != 240 || len(a.widths[layoutPractice]) != 0 || len(a.widths[layoutRace]) != 0 {
		t.Fatal("width groups mixed")
	}
	b := App{data: a.data, profileName: a.profileName}
	b.loadColumns()
	if b.widths[layoutQualifying]["driver"] != 240 {
		t.Fatal("width lost after restart")
	}
	b.profileName = "another"
	b.loadColumns()
	if len(b.widths[layoutQualifying]) != 0 {
		t.Fatal("width crossed profiles")
	}
	a.retroAction(desktopAction{Kind: "field", Key: "driver", Visible: false, Layout: layoutQualifying})
	a.retroAction(desktopAction{Kind: "field", Key: "driver", Visible: true, Layout: layoutQualifying})
	if a.widths[layoutQualifying]["driver"] != 240 {
		t.Fatal("hiding field lost its width")
	}
	for _, width := range []float32{-1, 31, 1201, float32(math.NaN()), float32(math.Inf(1))} {
		a.retroAction(desktopAction{Kind: "width", Key: "driver", Width: width, Layout: layoutQualifying})
	}
	a.retroAction(desktopAction{Kind: "width", Key: "driver", Width: 100, Layout: layoutRace})
	a.retroAction(desktopAction{Kind: "width", Key: "driver", Width: 100, Layout: layoutQualifying, PreferenceKey: "old-profile"})
	if a.widths[layoutQualifying]["driver"] != 240 {
		t.Fatal("invalid or stale width accepted")
	}
	a.retroAction(desktopAction{Kind: "width", Key: "driver", Width: 0, Layout: layoutQualifying})
	if len(a.widths[layoutQualifying]) != 0 {
		t.Fatal("automatic width reset failed")
	}
	a.retroAction(desktopAction{Kind: "width", Key: "driver", Width: 200, Layout: layoutQualifying})
	a.retroAction(desktopAction{Kind: "reset", Layout: layoutQualifying})
	if len(a.widths[layoutQualifying]) != 0 {
		t.Fatal("reset defaults retained custom widths")
	}
}
func TestColumnWidthPreferenceCompatibility(t *testing.T) {
	a := App{data: t.TempDir(), profileName: "profile"}
	raw := columnPreferences{Version: 1, Hidden: map[string][]string{"practice": {"last"}}, Widths: map[string]map[string]float32{"practice": {"driver": 180, "last": 240, "counter": 200, "unknown": 300, "tyre": -4}}}
	encoded, _ := json.Marshal(raw)
	if err := os.WriteFile(a.preferencePath(), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	a.loadColumns()
	if a.fieldVisible(layoutPractice, "last") || a.widths[layoutPractice]["last"] != 240 || len(a.widths[layoutPractice]) != 2 {
		t.Fatal("width validation or hidden preferences lost")
	}
	// An original visibility-only preference file needs no migration/reset.
	raw.Widths = nil
	encoded, _ = json.Marshal(raw)
	os.WriteFile(a.preferencePath(), encoded, 0600)
	b := App{data: a.data, profileName: a.profileName}
	b.loadColumns()
	if b.fieldVisible(layoutPractice, "last") || len(b.widths[layoutPractice]) != 0 {
		t.Fatal("legacy preferences incompatible")
	}
}
