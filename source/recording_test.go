package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func analysisPacket(format int, id byte, frame uint32, clock float32) []byte {
	if id != 6 && id != 8 && id != 15 {
		b := packet(format, id, 123, frame)
		putFloat(b[15:], clock)
		return b
	}
	slots := 22
	if format == 2026 {
		slots = 24
	}
	stride := 60
	if format == 2026 {
		stride = 59
	}
	size := 29 + slots*stride + 3
	if id == 8 {
		size = 30 + slots*46
	}
	if id == 15 {
		size = 31 + 50*slots
	}
	b := make([]byte, size)
	binary.LittleEndian.PutUint16(b, uint16(format))
	b[5], b[6] = 1, id
	binary.LittleEndian.PutUint64(b[7:], 123)
	binary.LittleEndian.PutUint32(b[23:], frame)
	putFloat(b[15:], clock)
	return b
}
func lapAt(format int, frame uint32, clock float32, lap int, distance float32) []byte {
	b := analysisPacket(format, 2, frame, clock)
	b[61], b[62] = 1, byte(lap)
	b[73] = 1 // flying lap
	putFloat(b[49:], distance)
	return b
}
func traceAt(format int, frame uint32, clock float32, speed int) []byte {
	b := analysisPacket(format, 6, frame, clock)
	binary.LittleEndian.PutUint16(b[29:], uint16(speed))
	putFloat(b[31:], .75)
	putFloat(b[39:], .2)
	b[44] = 5
	binary.LittleEndian.PutUint16(b[45:], 11000)
	return b
}
func eventAt(format int, frame uint32, clock float32, code string) []byte {
	b := analysisPacket(format, 3, frame, clock)
	copy(b[29:], code)
	return b
}

func TestRecordingPersistenceFormatsAndGrouping(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		t.Run(strconv.Itoa(format), func(t *testing.T) {
			data := t.TempDir()
			r, e := openRecorder(data)
			if e != nil {
				t.Fatal(e)
			}
			session := analysisPacket(format, 1, 1, 0)
			session[35], session[36] = 15, 4
			binary.LittleEndian.PutUint32(session[670:], 111)
			binary.LittleEndian.PutUint32(session[674:], 222)
			r.ingest(session, "career")
			lights := eventAt(format, 1, 0, "STLG")
			lights[33] = 1
			r.ingest(lights, "career")
			r.ingest(lapAt(format, 2, 1, 1, 100), "career")
			r.ingest(traceAt(format, 2, 1, 280), "career")
			if r.current == nil {
				t.Fatal("recording failed:", r.err)
			}
			id := r.current.ID
			if e = r.rename(id, "My Team"); e != nil {
				t.Fatal(e)
			}
			r.ingest(eventAt(format, 3, 2, "SEND"), "career")
			r.finish("interrupted")
			reopened, e := openRecorder(data)
			if e != nil || len(reopened.items) != 1 {
				t.Fatal(e, reopened)
			}
			meta := reopened.items[0]
			if meta.State != "completed" || meta.Save != "My Team" || meta.Season != 111 || meta.Weekend != 222 {
				t.Fatalf("bad metadata %+v", meta)
			}
			m, e := loadAnalysis(r.dir, meta)
			if e != nil || m.Truncated || len(m.Drivers[0].Samples) != 1 || m.Drivers[0].Samples[0].Speed != 280 {
				t.Fatalf("bad reload %+v %v", m, e)
			}
			binary.LittleEndian.PutUint64(session[7:], 456)
			reopened.ingest(session, "career")
			if reopened.current.Save != "My Team" {
				t.Fatal("season association not retained")
			}
			reopened.finish("interrupted")
			binary.LittleEndian.PutUint64(session[7:], 789)
			binary.LittleEndian.PutUint32(session[670:], 999)
			reopened.ingest(session, "career")
			if reopened.current.Save == "My Team" {
				t.Fatal("unrelated season silently merged")
			}
			reopened.finish("interrupted")
		})
	}
}
func TestRecorderRestartEndAndLatePackets(t *testing.T) {
	r, e := openRecorder(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	r.ingest(lapAt(2026, 1, 0, 1, 0), "p")
	r.ingest(eventAt(2026, 2, 0, "SSTA"), "p")
	first := r.current.ID
	r.ingest(eventAt(2026, 2, 0, "SSTA"), "p")
	if r.current.ID != first {
		t.Fatal("duplicate start split grid")
	}
	r.ingest(eventAt(2026, 20, 10, "SSTA"), "p")
	if len(r.items) != 1 || r.items[0].State != "restarted" {
		t.Fatal("restart not separate")
	}
	second := r.current.ID
	r.ingest(eventAt(2026, 21, 11, "SEND"), "p")
	before := r.current.Packets
	r.ingest(lapAt(2026, 22, 12, 1, 0), "p")
	if r.current.ID != second || r.current.Packets != before+1 || r.current.State != "completed" {
		t.Fatal("late data must be saved without reviving a completed session")
	}
	r.ingest(analysisPacket(2026, 11, 23, 12), "p")
	if r.current.Packets != before+2 {
		t.Fatal("late history lost")
	}
	r.finish("interrupted")
	r.ingest(lapAt(2026, 24, 13, 1, 0), "p")
	if r.current != nil {
		t.Fatal("retired UID reopened")
	}
}
func TestRecoverFlushedJournalWithoutFooter(t *testing.T) {
	r, e := openRecorder(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	r.ingest(lapAt(2025, 1, 1, 1, 100), "p")
	r.ingest(traceAt(2025, 1, 1, 123), "p")
	if e = r.flush(); e != nil {
		t.Fatal(e)
	}
	id := r.current.ID
	_ = r.file.Close() // simulate process crash, deliberately omit gzip.Close
	reopened, e := openRecorder(filepath.Dir(r.dir))
	if e != nil {
		t.Fatal(e)
	}
	if len(reopened.items) != 1 || reopened.items[0].State != "interrupted" {
		t.Fatal("crash not recovered")
	}
	m, e := loadAnalysis(r.dir, reopened.items[0])
	if e != nil || !m.Truncated || len(m.Drivers[0].Samples) != 1 {
		t.Fatalf("recovery failed %v", e)
	}
	if _, e = os.Stat(filepath.Join(r.dir, id+".udp.gz")); e != nil {
		t.Fatal(e)
	}
}
func TestPairingPacketLossAndReordering(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		m := newModel(RecordingMeta{Format: format})
		m.ingest(traceAt(format, 1, 1, 222))
		m.ingest(lapAt(format, 1, 1, 1, 100))
		m.ingest(traceAt(format, 3, 3, 333))
		m.ingest(lapAt(format, 4, 4, 2, 5))
		if len(m.Drivers[0].Samples) != 1 {
			t.Fatal("unpaired telemetry assigned to wrong lap")
		}
		s := m.Drivers[0].Samples[0]
		if s.Lap != 1 || s.Speed != 222 || s.Throttle < 74.9 || s.RPM != 11000 || s.Gear != 5 {
			t.Fatalf("bad sample %+v", s)
		}
	}
}
func TestFlashbackRewindsTracesPitsEventsAndSafety(t *testing.T) {
	m := newModel(RecordingMeta{Format: 2026})
	session := analysisPacket(2026, 1, 1, 0)
	session[153] = 1
	m.ingest(session)
	for i := 1; i <= 3; i++ {
		b := lapAt(2026, uint32(i), float32(i), 1, float32(i*100))
		b[75] = 1
		binary.LittleEndian.PutUint16(b[76:], uint16(i*1000))
		binary.LittleEndian.PutUint16(b[78:], uint16(i*500))
		m.ingest(b)
		m.ingest(traceAt(2026, uint32(i), float32(i), 200+i))
		if i == 3 {
			m.ingest(eventAt(2026, 30, 3, "OVTK"))
		}
	}
	m.rewind(1.5)
	d := &m.Drivers[0]
	if len(d.Samples) != 1 || len(d.Pits) != 1 || d.Pits[0].LaneMS != 1000 || d.Pits[0].StopMS != 500 || len(m.Events) != 0 || !m.SCLaps[1] {
		t.Fatalf("bad rewind samples %d pits %+v events %+v SC %+v", len(d.Samples), d.Pits, m.Events, m.SCLaps)
	}
}
func TestHistoryClassificationPitAndPositionDecoding(t *testing.T) {
	m := newModel(RecordingMeta{Format: 2025})
	h := analysisPacket(2025, 11, 1, 0)
	h[30], h[31] = 1, 1
	binary.LittleEndian.PutUint32(h[36:], 90123)
	binary.LittleEndian.PutUint16(h[40:], 30000)
	binary.LittleEndian.PutUint16(h[43:], 30123)
	binary.LittleEndian.PutUint16(h[46:], 30000)
	h[49] = 15
	h[1436], h[1437] = 5, 17
	m.ingest(h)
	l := m.Drivers[0].Laps[1]
	if !l.Valid || l.Time != 90123 || l.S1 != 30000 || l.S2 != 30123 || l.S3 != 30000 {
		t.Fatalf("bad sectors %+v", l)
	}
	pos := analysisPacket(2025, 15, 2, 1)
	pos[29], pos[30], pos[31] = 1, 0, 4
	m.ingest(pos)
	if l.Position != 4 {
		t.Fatal("lap-start position")
	}
	d := &m.Drivers[0]
	d.applyPit(pitPoint{1, 5, 17, true, 1000, 0})
	d.applyPit(pitPoint{2, 5, 16, true, 24000, 2500})
	d.applyPit(pitPoint{3, 5, 16, false, 24000, 2500})
	if !reflect.DeepEqual(d.Pits, []PitVisit{{Lap: 5, Time: 1, CompoundIn: 17, CompoundOut: 16, LaneMS: 24000, StopMS: 2500, Complete: true}}) {
		t.Fatal(d.Pits)
	}
}
