package main

import (
	"encoding/json"
	"testing"
)

// The launcher requires a tone on Block::Text. Check all nested panels as
// well as the waiting/live/error pages, which exposed the v0.1.0 failure.
func TestRequiredTextTone(t *testing.T) {
	a := App{enabled: true, prefs: Preferences{Mode: "full", Port: 20777}}
	for _, state := range []string{"waiting", "live", "error"} {
		if state == "live" {
			a.expandedSet = true
			a.expanded = 0
			a.tel.Active = true
			a.tel.Cars[0] = Car{Participant: true, Name: "Player", HasStatus: true, Compound: 18, Visual: 16}
			a.tel.Cars[1] = Car{Participant: true, Name: "Rival", Setup: make([]byte, 50)}
		}
		if state == "error" {
			a.listenErr = "Port is occupied"
		}
		raw, err := json.Marshal(a.telemetryView())
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err = json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		count := 0
		var check func(any)
		check = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				if txt, ok := x["text"].(map[string]any); ok {
					if txt["tone"] != "neutral" {
						t.Fatalf("%s: text block lacks required tone: %v", state, txt)
					}
					count++
				}
				for _, child := range x {
					check(child)
				}
			case []any:
				for _, child := range x {
					check(child)
				}
			}
		}
		check(v)
		if count == 0 {
			t.Fatalf("%s: no text blocks checked", state)
		}
	}
}
