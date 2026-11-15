package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type traceLapMark struct {
	Time       float64
	Frame, Lap int
}

func (d *RecordedDriver) noteTraceLap(s TraceSample) {
	marks := d.traceLaps
	n := sort.Search(len(marks), func(i int) bool {
		return marks[i].Time > s.Time || marks[i].Time == s.Time && marks[i].Frame >= s.Frame
	})
	if n < len(marks) && marks[n].Time == s.Time && marks[n].Frame == s.Frame {
		return
	}
	mark := traceLapMark{s.Time, s.Frame, s.Lap}
	marks = append(marks, traceLapMark{})
	copy(marks[n+1:], marks[n:])
	marks[n] = mark
	d.traceLaps = marks
	// Keep the recent exact-frame pairing window uncompressed: reordered
	// packets near a timing line may insert a lap boundary between samples.
	// Older completed runs need only their lap-transition markers.
	if s.Frame > d.traceCompactFrame+32 {
		kept := marks[:0]
		for _, m := range marks {
			if m.Frame >= s.Frame-64 || len(kept) == 0 || kept[len(kept)-1].Lap != m.Lap {
				kept = append(kept, m)
			}
		}
		d.traceLaps = kept
		d.traceCompactFrame = s.Frame
	}
}
func (d *RecordedDriver) traceLapAt(clock float64) int {
	n := sort.Search(len(d.traceLaps), func(i int) bool { return d.traceLaps[i].Time > clock })
	if n > 0 {
		return d.traceLaps[n-1].Lap
	}
	return 0
}
func (d *RecordedDriver) trimTraceLaps(clock float64, frame *uint32) {
	n := sort.Search(len(d.traceLaps), func(i int) bool {
		m := d.traceLaps[i]
		return m.Time > clock || m.Time == clock && frame != nil && m.Frame > int(*frame)
	})
	d.traceLaps = d.traceLaps[:n]
	d.traceCompactFrame = 0
}
func (m *AnalysisModel) pairingPoints(frame uint32, slots int) []lapPoint {
	// Recycle an expired pairing frame. The 64-frame delayed-packet window and
	// exact-frame pairing remain intact, without one allocation per lap packet.
	for f, points := range m.lapFrames {
		if f < frame && frame-f > 64 && len(points) == slots {
			delete(m.lapFrames, f)
			clear(points)
			return points
		}
	}
	return make([]lapPoint, slots)
}

// nil scope retains the original full replay for diagnostics/legacy callers.
// Interactive analysis needs timings for the whole session, but driving samples
// only for the selected race lap and selected drivers.
type analysisTraceScope struct {
	Lap     int
	Drivers uint32
	Full    bool
}

// Completed analysis should not regenerate and transmit large chart views every
// second. User events and loader completion still invalidate immediately; this
// stamp also discovers new recordings, renames and late UDP changes.
func (a *App) analysisDataStampLocked() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d/%s/%s/%s", a.analysis.Generation, a.analysis.Loading, a.analysis.Error, a.recordingErr)
	if a.rec != nil {
		fmt.Fprintf(&b, "/%s", a.rec.err)
		for _, m := range a.rec.catalog() {
			fmt.Fprintf(&b, "/%s:%s:%s:%d:%d:%s", m.ID, m.State, m.Save, m.Session, m.Track, m.Error)
			if m.State == "recording" {
				fmt.Fprintf(&b, ":%d", m.Packets)
			}
		}
	}
	return b.String()
}
func (a *App) analysisNeedsInvalidate() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.analysis.DataStamp != a.analysisDataStampLocked()
}

func (s analysisTraceScope) captures(driver, lap int) bool {
	return s.Full || s.Lap > 0 && lap == s.Lap && s.Drivers&(1<<uint(driver)) != 0
}
func (s analysisTraceScope) hasTraces() bool { return s.Full || s.Lap > 0 && s.Drivers != 0 }
func (m *AnalysisModel) coversScope(wanted analysisTraceScope) bool {
	if m.traceScope == nil || m.traceScope.Full {
		return true
	}
	have := *m.traceScope
	return !wanted.hasTraces() || !wanted.Full && have.Lap == wanted.Lap && have.Drivers&wanted.Drivers == wanted.Drivers
}
func (m *AnalysisModel) releaseUnusedTraces(wanted analysisTraceScope) {
	if wanted.Full {
		return
	}
	for i := range m.Drivers {
		if !wanted.hasTraces() || wanted.Drivers&(1<<uint(i)) == 0 {
			m.Drivers[i].Samples = nil
		}
	}
	// Only interactive scoped models are trimmed. Diagnostic full models retain
	// their original contract, including multiple laps, for existing callers.
	if m.traceScope != nil {
		scope := wanted
		m.traceScope = &scope
	}
}
func (a *App) wantedTraceScope(meta RecordingMeta) analysisTraceScope {
	s := &a.analysis
	if s.Tab != "traces" || practiceSession(meta.Session) || qualifyingSession(meta.Session) {
		return analysisTraceScope{}
	}
	if !raceSession(meta.Session) {
		return analysisTraceScope{Full: true}
	}
	drivers := s.RaceDrivers
	if s.RaceSelectionSession != meta.ID {
		drivers = (1 << 24) - 1
		if meta.Player >= 0 && meta.Player < 24 {
			drivers = 1 << uint(meta.Player)
		}
	}
	return analysisTraceScope{Lap: max(1, s.RaceLap), Drivers: drivers}
}
func loadAnalysisScoped(ctx context.Context, dir string, meta RecordingMeta, scope analysisTraceScope) (*AnalysisModel, error) {
	m := newModel(meta)
	m.traceScope = &scope
	partial, e := readJournalContext(ctx, filepath.Join(dir, meta.ID+".udp.gz"), true, m.ingest)
	m.Truncated = partial
	return m, e
}
