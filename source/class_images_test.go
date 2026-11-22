package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestClassIndependentServiceImages(t *testing.T) {
	png := testBadgePNG(t)
	for _, session := range []uint32{1, 5, 10, 15, 16, 17, 18} {
		for compound := int32(0); compound < 255; compound++ {
			a := App{frame: badgeFrame(), mode: "full", status: "Live"}
			a.frame.SessionType = session
			a.frame.SessionKind = 3
			c := &a.frame.Cars[0]
			c.Compound = compound
			c.Visual = 254 - compound
			c.BadgePNG = png
			c.TyrePNG = png
			cells := driverCells(*c, layoutFor(a.frame), 0, false)
			for i, col := range columns(layoutFor(a.frame)) {
				if col["key"] == "team" || col["key"] == "tyre" {
					raw, _ := json.Marshal(cells[i])
					if !bytes.Contains(raw, []byte("image_data")) {
						t.Fatalf("session %d compound %d image %s hidden", session, compound, col["key"])
					}
				}
			}
			a.displayMode = "window"
			retro := a.retroModel(true)
			for i, col := range retro.Columns {
				if (col.Key == "team" || col.Key == "tyre") && retro.Rows[0].Cells[i].PNG != png {
					t.Fatalf("retro class image hidden %d/%d", session, compound)
				}
			}
		}
	}
}

func TestImagesDoNotDependOnOtherTelemetry(t *testing.T) {
	c := badgeFrame().Cars[0]
	c.BadgePNG = testBadgePNG(t)
	c.TyrePNG = c.BadgePNG
	// Service images are validated by identity/generation in the shared reader.
	// Their display does not require tyre numbers or current classification.
	c.Known = knownName
	c.Position = 0
	c.Compound = 255
	c.Visual = 255
	if badgeCell(c) == "" || tyreCell(c) == "—" {
		t.Fatal("optional telemetry hid available images")
	}
	if _, exists := tyreCell(c).(M)["sort"]; exists {
		t.Fatal("unknown compound fabricated a sort value")
	}
	c.TyrePNG = ""
	if badgeCell(c) == "" || tyreCell(c) != "—" {
		t.Fatal("missing tyre image hid team badge")
	}
	c.TyrePNG = c.BadgePNG
	c.BadgePNG = ""
	if badgeCell(c) != "" || tyreCell(c) == "—" {
		t.Fatal("missing team badge hid tyre image")
	}
	c.Known = 0
	if badgeCell(c) != "" || tyreCell(c) != "—" {
		t.Fatal("image appeared before driver identity data")
	}
	c.Known = knownName
	c.Identity = 0
	if badgeCell(c) != "" || tyreCell(c) != "—" {
		t.Fatal("unidentified car showed images")
	}
}
