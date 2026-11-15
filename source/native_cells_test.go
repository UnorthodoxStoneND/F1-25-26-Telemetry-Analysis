package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestNativeTyresUseActualLabelAndVisualColour(t *testing.T) {
	for _, tc := range []struct {
		actual, visual int
		label, colour  string
	}{
		{20, 18, "C1", "#EEF2FA"},
		{19, 18, "C2", "#EEF2FA"}, {19, 17, "C2", "#FFD53F"},
		{18, 18, "C3", "#EEF2FA"}, {18, 17, "C3", "#FFD53F"}, {18, 16, "C3", "#FF4A5B"},
		{17, 17, "C4", "#FFD53F"}, {17, 16, "C4", "#FF4A5B"}, {16, 16, "C5", "#FF4A5B"},
		{7, 0, "INT", "#41DA8B"}, {8, 0, "WET", "#4B9FFF"},
		{18, 0, "C3", ""},
	} {
		cell := tyreCell(Car{HasStatus: true, Compound: tc.actual, Visual: tc.visual})
		if cell["text"] != tc.label || cell["image"] != nil {
			t.Fatal(tc, cell)
		}
		if tc.colour == "" {
			if cell["edit"] != nil {
				t.Fatal("unknown visual compound acquired a colour", cell)
			}
			continue
		}
		field := cell["edit"].(M)["colour"].(M)
		if field["value"] != tc.colour || field["disabled"] != true || field["hex"] == false {
			t.Fatal("incorrect or editable tyre circle", tc, cell)
		}
	}
	for _, car := range []Car{{}, {HasStatus: true, Compound: 255}} {
		cell := tyreCell(car)
		if cell["text"] != "—" || cell["edit"] != nil {
			t.Fatal(cell)
		}
	}
}

func TestNativeMedalsAndNumericPositionSort(t *testing.T) {
	a := App{}
	for n, medal := range []string{"🥇", "🥈", "🥉"} {
		cell := a.podiumPosition(n + 1).(M)
		if cell["text"] != medal || cell["sort"] != n+1 || cell["image"] != nil {
			t.Fatal(cell)
		}
	}
	if a.podiumPosition(4) != "4" || a.podiumPosition(0) != "—" {
		t.Fatal("numeric positions changed")
	}
}

func TestReleasedAndFutureLauncherHandshake(t *testing.T) {
	t.Setenv("F1_TELEMETRY_TEST_DOCUMENTS", t.TempDir())
	for _, tc := range []struct {
		version      string
		capabilities []string
		supported    bool
	}{
		{"1.9.55", []string{"view.chart"}, true},
		{"1.9.99", []string{"view.chart"}, true},
		{"2.0.0", []string{"other", "view.chart"}, true},
		{"1.9.55", nil, false},
		{"9.0.0", []string{"view.chart.future"}, false},
	} {
		var input, output bytes.Buffer
		encoder := json.NewEncoder(&input)
		_ = encoder.Encode(M{"id": 1, "method": "hello", "params": M{"api": 1, "data_dir": t.TempDir(), "launcher": tc.version, "capabilities": tc.capabilities}})
		_ = encoder.Encode(M{"id": 2, "method": "view", "params": M{"section": "analysis"}})
		_ = encoder.Encode(M{"id": 100001, "result": M{"running": false}})
		if err := run(&input, &output); err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(&output)
		sawView := false
		for decoder.More() {
			var message M
			if err := decoder.Decode(&message); err != nil {
				t.Fatal(err)
			}
			if message["error"] != nil {
				t.Fatal(message)
			}
			if message["id"] == float64(2) {
				sawView = true
				unsupported := strings.Contains(fmt.Sprint(message["result"]), "view.chart")
				if unsupported == tc.supported {
					t.Fatal(tc, message)
				}
			}
		}
		if !sawView {
			t.Fatal("missing analysis reply", tc)
		}
	}
}
