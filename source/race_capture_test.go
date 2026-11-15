package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"
)

func raceGridPair(format int, frame uint32, clock float32, timer uint32, distance float32, speed int) ([]byte, []byte) {
	lap, car := analysisPacket(format, 2, frame, clock), traceAt(format, frame, clock, speed)
	n, stride := 22, 60
	if format == 2026 {
		n, stride = 24, 59
	}
	for i := 0; i < n; i++ {
		r := lap[29+i*57:]
		r[32], r[33], r[44] = byte(i+1), 1, 4
		binary.LittleEndian.PutUint32(r[4:], timer)
		putFloat(r[20:], distance-float32(i)*.5)
		c := car[29+i*stride:]
		binary.LittleEndian.PutUint16(c, uint16(speed))
		binary.LittleEndian.PutUint16(c[16:], 12345)
	}
	return lap, car
}
func TestRaceRecordingStartsAtRedLightsForEveryCar(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		for _, sessionType := range []int{15, 16, 17} {
			t.Run(fmt.Sprintf("%d/session%d", format, sessionType), func(t *testing.T) {
				r, e := openRecorder(t.TempDir())
				if e != nil {
					t.Fatal(e)
				}
				defer r.finish("interrupted")
				session := analysisPacket(format, 1, 0, 0)
				session[35], session[36], session[153] = byte(sessionType), 0, 3
				r.ingest(session, "career")
				r.ingest(eventAt(format, 0, 0, "LGOT"), "career")
				n, stride, name := 22, 57, 7
				if format == 2026 {
					n, stride, name = 24, 60, 10
				}
				participants := analysisPacket(format, 4, 1, 0)
				participants[29] = byte(n)
				for i := 0; i < n; i++ {
					row := participants[30+i*stride:]
					row[0] = 1
					copy(row[name:], fmt.Sprintf("Grid Driver %d", i))
				}
				r.ingest(participants, "career")
				formationLap, formationCar := raceGridPair(format, 50, 140, 0, 5000, 77)
				r.ingest(formationLap, "career")
				r.ingest(formationCar, "career")
				if r.current.Packets != 0 {
					t.Fatal("formation driving data written before lights", r.current.Packets)
				}
				session = analysisPacket(format, 1, 95, 0)
				session[35], session[36] = byte(sessionType), 0
				r.ingest(session, "career")
				lights := eventAt(format, 100, 0, "STLG")
				lights[33] = 1
				r.ingest(lights, "career")
				for _, frame := range []uint32{101, 102} {
					lap, car := raceGridPair(format, frame, 0, 0, -100, 0)
					r.ingest(lap, "career")
					r.ingest(car, "career")
				}
				r.ingest(eventAt(format, 105, 0, "LGOT"), "career")
				lap, car := raceGridPair(format, 106, 1, 1000, -50, 100)
				r.ingest(lap, "career")
				r.ingest(car, "career")
				lap, car = raceGridPair(format, 200, 90, 90000, 5270, 280)
				r.ingest(lap, "career")
				r.ingest(car, "career")
				r.ingest(eventAt(format, 201, 91, "SEND"), "career")
				meta := *r.current
				if meta.RaceCaptureFrame == nil || *meta.RaceCaptureFrame != 100 || meta.RaceStartLights != 1 || meta.RaceCaptureTrigger != "start_lights" {
					t.Fatal("capture marker not persisted", meta)
				}
				if e = r.flush(); e != nil {
					t.Fatal(e)
				}
				_, e = readJournal(filepath.Join(r.dir, meta.ID+".udp.gz"), func(b []byte) {
					if (b[6] == 2 || b[6] == 6) && u32(b[23:]) < 100 {
						t.Fatal("formation measurement in journal")
					}
				})
				if e != nil {
					t.Fatal(e)
				}
				m, e := loadAnalysis(r.dir, meta)
				if e != nil {
					t.Fatal(e)
				}
				for i := 0; i < n; i++ {
					samples := m.Drivers[i].trace(1)
					if len(samples) != 4 || samples[0].Frame != 101 || samples[1].Frame != 102 || samples[0].RPM != 12345 || samples[0].Distance >= 0 || samples[3].Distance < 5200 {
						t.Fatalf("car %d lost red-light or lap-one data: %+v", i, samples)
					}
					if m.Drivers[i].Name != fmt.Sprintf("Grid Driver %d", i) {
						t.Fatal("cached participants were lost")
					}
				}
			})
		}
	}
}
func TestRaceStartMissingEventsAndLateJoin(t *testing.T) {
	for _, format := range []int{2025, 2026} {
		r, e := openRecorder(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		session := analysisPacket(format, 1, 1, 0)
		session[35], session[36], session[153] = 15, 0, 3
		r.ingest(session, "career")
		r.ingest(eventAt(format, 2, 0, "LGOT"), "career")
		if r.raceGate.Open {
			t.Fatal("formation LGOT opened capture")
		}
		session[153] = 0
		r.ingest(session, "career")
		r.ingest(eventAt(format, 20, 0, "LGOT"), "career")
		lap, car := raceGridPair(format, 21, 0, 0, -100, 0)
		r.ingest(lap, "career")
		r.ingest(car, "career")
		if r.current.RaceCaptureTrigger != "lights_out_fallback" {
			t.Fatal("lost STLG fallback", r.current.RaceCaptureTrigger)
		}
		if e = r.flush(); e != nil {
			t.Fatal(e)
		}
		m, e := loadAnalysis(r.dir, *r.current)
		if e != nil || len(m.Drivers[0].Samples) != 1 {
			t.Fatal("fallback boundary not replayable", e)
		}
		r.finish("interrupted")
		g := RaceCaptureGate{}
		lap, _ = raceGridPair(format, 800, 40, 40000, 2500, 250)
		if !g.allow(lap, 15) || g.Trigger != "timed_lap_fallback" {
			t.Fatal("joining an already running race failed")
		}
	}
	for _, session := range []int{1, 5, 8, 14} {
		g := RaceCaptureGate{}
		if !g.allow(traceAt(2026, 1, 0, 50), session) {
			t.Fatal("non-race was start-gated", session)
		}
	}
}
func TestFlashbackBeforeRedLightsRearmsCapture(t *testing.T) {
	m := newModel(RecordingMeta{Format: 2026, Session: 15})
	lights := eventAt(2026, 100, 0, "STLG")
	lights[33] = 1
	m.ingest(lights)
	lap, car := raceGridPair(2026, 101, 0, 0, -100, 0)
	m.ingest(lap)
	m.ingest(car)
	rewind := eventAt(2026, 200, 0, "FLBK")
	binary.LittleEndian.PutUint32(rewind[33:], 99)
	putFloat(rewind[37:], 0)
	m.ingest(rewind)
	if m.raceGate.Open || len(m.Drivers[0].Samples) != 0 {
		t.Fatal("old zero-clock start samples survived flashback")
	}
	lap, car = raceGridPair(2026, 99, 0, 0, -100, 77)
	m.ingest(lap)
	m.ingest(car)
	if len(m.Drivers[0].Samples) != 0 {
		t.Fatal("pre-light replay was included")
	}
	m.ingest(lights)
	lap, car = raceGridPair(2026, 101, 0, 0, -100, 10)
	m.ingest(lap)
	m.ingest(car)
	if len(m.Drivers[0].Samples) != 1 || m.Drivers[0].Samples[0].Speed != 10 {
		t.Fatal("new start attempt not captured")
	}
}
