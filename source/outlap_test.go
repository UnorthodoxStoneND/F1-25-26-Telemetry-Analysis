package main

import "testing"

func TestOutlapStatusInBothViews(t *testing.T) {
	for session := uint32(1); session <= 17; session++ {
		a := App{frame: grid(), mode: "full", status: "Live"}
		a.frame.SessionType = session
		c := &a.frame.Cars[0]
		c.Known |= knownResult | knownPit
		c.Flags = 2 << 8
		a.lastView, a.viewSent = a.viewState(), true
		c.Flags |= outLap
		want := "Out"
		if session >= 15 {
			want = "Run"
		}
		if got := cellText(t, rowCells(t, &a, c.Vehicle)["status"]); got != want {
			t.Fatalf("session %d status %s want %s", session, got, want)
		}
		m := a.retroModel(true)
		column := -1
		for i, col := range m.Columns {
			if col.Key == "status" {
				column = i
			}
		}
		if column < 0 || len(m.Rows) == 0 || m.Rows[0].Cells[column].Text != want {
			t.Fatalf("retro session %d status mismatch", session)
		}
		if session < 15 && !a.needsView() {
			t.Fatal("OUT did not refresh")
		}
		a.lastView = a.viewState()
		c.Flags &^= outLap
		if session < 15 && !a.needsView() {
			t.Fatal("RUN did not refresh")
		}
	}
}

func TestOutlapKeepsPitAndResultsPriority(t *testing.T) {
	for _, test := range []struct {
		result uint32
		pit    bool
		want   string
	}{
		{2, false, "Out"}, {2, true, "Pit"}, {4, false, "DNF"}, {7, true, "DNF"}, {5, true, "DSQ"}, {3, false, "Finished"}, {6, false, "NC"},
	} {
		c := Car{Known: knownPit | knownResult, Flags: outLap | test.result<<8}
		if test.pit {
			c.Flags |= inPitLane
		}
		if got := cellText(t, statusCell(c)); got != test.want {
			t.Fatalf("status %s want %s", got, test.want)
		}
	}
	c := Car{Flags: outLap}
	if statusCell(c) != "—" {
		t.Fatal("unknown result fabricated status")
	}
}
