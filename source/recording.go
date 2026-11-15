package main

// Record the original UDP datagrams, rather than only the fields currently drawn
// by the UI. A flushed gzip journal can be read up to its last complete record
// after an interrupted write. This also permits correcting the decoder later.
import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type RecordingMeta struct {
	ID                           string `json:"id"`
	UID                          uint64 `json:"udp_uid"`
	Format                       int    `json:"format"`
	Profile                      string `json:"profile"`
	Save                         string `json:"save"`
	Season, Weekend, SessionLink uint32
	Session, Track, Player       int
	WeekendStructure             []int `json:"weekend_structure,omitempty"`
	Started, Updated             time.Time
	State                        string        `json:"state"`
	Packets                      int64         `json:"packets"`
	Error                        string        `json:"error,omitempty"`
	ClassificationTime           *float64      `json:"classification_time,omitempty"`
	SessionEndTime               *float64      `json:"session_end_time,omitempty"`
	LastTelemetryTime            *float64      `json:"last_telemetry_time,omitempty"`
	TelemetryAfterClassification int64         `json:"telemetry_packets_after_classification,omitempty"`
	TelemetryAfterEnd            int64         `json:"telemetry_packets_after_session_end,omitempty"`
	Teams                        *TeamSnapshot `json:"team_snapshot,omitempty"`
	TeamCaptureError             string        `json:"team_capture_error,omitempty"`
	RaceCaptureFrame             *uint32       `json:"race_capture_frame,omitempty"`
	RaceCaptureTrigger           string        `json:"race_capture_trigger,omitempty"`
	RaceStartLights              int           `json:"race_start_lights,omitempty"`
}
type Recorder struct {
	dir            string
	items          []RecordingMeta
	current        *RecordingMeta
	file           *os.File
	zip            *gzip.Writer
	lastFlush      time.Time
	flushedPackets int64
	started        bool
	startFrame     uint32
	retired        map[uint64]bool
	err            string
	lastUID        uint64
	raceGate       RaceCaptureGate
	preRace        map[byte][]byte
}

func atomicJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := path + ".tmp"
	f, e := os.Create(tmp)
	if e != nil {
		return e
	}
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	c := f.Close()
	if e == nil {
		e = c
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func openRecorder(data string) (*Recorder, error) {
	r := &Recorder{dir: filepath.Join(data, "recordings"), retired: map[uint64]bool{}}
	if e := os.MkdirAll(r.dir, 0755); e != nil {
		return nil, e
	}
	paths, e := filepath.Glob(filepath.Join(r.dir, "*.json"))
	if e != nil {
		return nil, e
	}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		var m RecordingMeta
		if json.Unmarshal(b, &m) != nil || !validRecordingID(m.ID) {
			continue
		}
		if m.State == "recording" {
			m.State = "interrupted"
			_ = atomicJSON(p, m)
		}
		if len(m.WeekendStructure) == 0 && raceSession(m.Session) {
			m.WeekendStructure = journalWeekendStructure(filepath.Join(r.dir, m.ID+".udp.gz"))
		}
		r.items = append(r.items, m)
	}
	sort.Slice(r.items, func(i, j int) bool { return r.items[i].Started.Before(r.items[j].Started) })
	return r, nil
}
func validRecordingID(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil
}
func (r *Recorder) persist() error {
	if r.current == nil {
		return nil
	}
	return atomicJSON(filepath.Join(r.dir, r.current.ID+".json"), r.current)
}
func (r *Recorder) flush() error {
	if r.zip == nil {
		return nil
	}
	// Seal completed-session members so an analysis snapshot has a genuine
	// gzip footer. Any late same-session UDP can append another gzip member.
	if r.current.State == "completed" {
		if e := r.zip.Close(); e != nil {
			return e
		}
		r.zip = nil
	} else {
		if e := r.zip.Flush(); e != nil {
			return e
		}
	}
	if e := r.file.Sync(); e != nil {
		return e
	}
	r.lastFlush = time.Now()
	r.flushedPackets = r.current.Packets
	return r.persist()
}
func (r *Recorder) finish(state string) {
	if r.current == nil {
		return
	}
	if r.current.State == "recording" {
		r.current.State = state
	}
	var e error
	if r.zip != nil {
		e = r.zip.Close()
		r.zip = nil
	}
	if r.file != nil {
		if x := r.file.Sync(); e == nil {
			e = x
		}
		if x := r.file.Close(); e == nil {
			e = x
		}
		r.file = nil
	}
	if e != nil {
		r.err = e.Error()
		r.current.Error = r.err
		r.current.State = "interrupted"
	}
	if e = r.persist(); e != nil {
		r.err = e.Error()
	}
	r.items = append(r.items, *r.current)
	r.lastUID = r.current.UID
	r.retired[r.current.UID] = true
	r.current = nil
	r.started = false
}
func (r *Recorder) begin(uid uint64, format, player int, profile string) error {
	var key [16]byte
	if _, e := rand.Read(key[:]); e != nil {
		return e
	}
	id := hex.EncodeToString(key[:])
	f, e := os.Create(filepath.Join(r.dir, id+".udp.gz"))
	if e != nil {
		return e
	}
	z, e := gzip.NewWriterLevel(f, gzip.BestSpeed)
	if e != nil {
		f.Close()
		return e
	}
	r.current = &RecordingMeta{ID: id, UID: uid, Format: format, Profile: profile, Save: "Unassigned " + id[:6], Player: player, Track: -1, Started: time.Now().UTC(), Updated: time.Now().UTC(), State: "recording"}
	r.file, r.zip = f, z
	r.started = false
	r.lastFlush = time.Time{}
	r.flushedPackets = -1
	r.raceGate = RaceCaptureGate{}
	r.preRace = map[byte][]byte{}
	return r.persist()
}
func (r *Recorder) ingest(b []byte, profile string) {
	if r == nil || len(b) < 29 || len(b) > 8192 || b[5] != 1 {
		return
	}
	format := int(u16(b))
	if format != 2025 && format != 2026 {
		return
	}
	id := b[6]
	if id > 16 {
		return
	}
	uid := binary.LittleEndian.Uint64(b[7:])
	code := ""
	if id == 3 && len(b) >= 33 {
		code = string(b[29:33])
	}
	if uid == 0 {
		if r.current == nil || r.current.Format != format {
			return
		}
		if code == "SEND" {
			// Shutdown events can reset the UID. Attribute the event to the
			// current recording, but retain its original bytes in the journal.
			uid = r.current.UID
		} else {
			if id == 1 && len(b) >= 37 && r.current.Session > 0 && b[35] == 0 && int8(b[36]) < 0 {
				r.finish("interrupted")
			}
			return
		}
	}
	if r.current == nil || r.current.UID != uid {
		if code == "SSTA" && r.current == nil && uid == r.lastUID {
			delete(r.retired, uid)
		}
		if r.retired[uid] || code == "SEND" {
			return
		}
		r.finish("interrupted")
		if e := r.begin(uid, format, int(b[27]), profile); e != nil {
			r.err = e.Error()
			return
		}
	}
	if r.current.Format != format {
		return
	}
	frame := u32(b[23:])
	if int(b[27]) < 24 {
		r.current.Player = int(b[27])
	}
	if code == "SSTA" {
		if r.current.State == "completed" || (r.started && r.startFrame != frame) {
			r.finish("restarted")
			delete(r.retired, uid)
			if e := r.begin(uid, format, int(b[27]), profile); e != nil {
				r.err = e.Error()
				return
			}
		}
		r.started = true
		r.startFrame = frame
	}
	// Results and SEND mark completion; they are not capture cutoffs. Keep all
	// same-session datagrams until explicit menu/new-session/game-exit evidence
	// closes the journal. Live Telemetry's separate end handling is unchanged.
	p := b[29:]
	if id == 1 && len(p) >= 653 && p[6] > 0 {
		r.current.Session, r.current.Track = int(p[6]), int(int8(p[7]))
		if structure := packetWeekendStructure(p); len(structure) > 0 {
			r.current.WeekendStructure = structure
		}
		r.current.Season, r.current.Weekend, r.current.SessionLink = u32(p[641:]), u32(p[645:]), u32(p[649:])
		// Adopt an explicitly named save for this profile+season, if known. Do not
		// infer a career from track, launcher profile alone, or driver names.
		if r.current.Season != 0 {
			for i := len(r.items) - 1; i >= 0; i-- {
				m := r.items[i]
				if m.Profile == profile && m.Season == r.current.Season {
					r.current.Save = m.Save
					break
				}
			}
		}
	}
	if !r.raceGate.allow(b, r.current.Session) {
		return
	}
	if raceSession(r.current.Session) {
		if !r.raceGate.Open && (id == 1 || id == 4 || id == 5 || id == 7) {
			if r.preRace == nil {
				r.preRace = map[byte][]byte{}
			}
			r.preRace[id] = append([]byte(nil), b...)
			return
		}
		if r.raceGate.Open {
			if r.current.RaceCaptureFrame == nil || *r.current.RaceCaptureFrame != r.raceGate.Frame {
				start := r.raceGate.Frame
				r.current.RaceCaptureFrame = &start
				r.current.RaceCaptureTrigger = r.raceGate.Trigger
				r.current.RaceStartLights = r.raceGate.Lights
			}
			for _, kind := range []byte{1, 4, 5, 7} {
				if cached := r.preRace[kind]; cached != nil {
					if e := r.appendDatagram(cached); e != nil {
						r.fail(e)
						return
					}
				}
			}
			r.preRace = nil
		}
	}
	if e := r.appendDatagram(b); e != nil {
		r.fail(e)
		return
	}
	classification := id == 8 && len(p) >= 1 && p[0] > 0 && int(p[0]) <= 24 && len(p) >= 1+int(p[0])*46
	clock := f32(b[15:])
	if finite(clock) {
		if classification && r.current.ClassificationTime == nil {
			v := clock
			r.current.ClassificationTime = &v
		}
		if code == "SEND" && r.current.SessionEndTime == nil {
			v := clock
			r.current.SessionEndTime = &v
		}
		if id == 6 {
			v := clock
			r.current.LastTelemetryTime = &v
			if r.current.ClassificationTime != nil {
				r.current.TelemetryAfterClassification++
			}
			if r.current.SessionEndTime != nil {
				r.current.TelemetryAfterEnd++
			}
		}
	}
	if code == "SEND" || classification {
		r.current.State = "completed"
	}
	if code == "SEND" || classification || time.Since(r.lastFlush) >= time.Second {
		if e := r.flush(); e != nil {
			r.fail(e)
		}
	}
}
func (r *Recorder) appendDatagram(b []byte) error {
	var n [2]byte
	binary.LittleEndian.PutUint16(n[:], uint16(len(b)))
	if r.zip == nil {
		z, e := gzip.NewWriterLevel(r.file, gzip.BestSpeed)
		if e != nil {
			return e
		}
		r.zip = z
	}
	if _, e := r.zip.Write(n[:]); e != nil {
		return e
	}
	if _, e := r.zip.Write(b); e != nil {
		return e
	}
	r.current.Packets++
	r.current.Updated = time.Now().UTC()
	return nil
}
func (r *Recorder) fail(e error) {
	r.err = e.Error()
	if r.current != nil {
		r.current.Error = r.err
	}
	r.finish("interrupted")
}
func (r *Recorder) catalog() []RecordingMeta {
	if r == nil {
		return nil
	}
	v := append([]RecordingMeta(nil), r.items...)
	if r.current != nil {
		v = append(v, *r.current)
	}
	return v
}
func (r *Recorder) rename(id, name string) error {
	if len(name) < 1 || len(name) > 80 {
		return fmt.Errorf("save name must be 1–80 characters")
	}
	var target *RecordingMeta
	if r.current != nil && r.current.ID == id {
		target = r.current
	}
	for i := range r.items {
		if r.items[i].ID == id {
			target = &r.items[i]
		}
	}
	if target == nil {
		return fmt.Errorf("recording not found")
	}
	profile, season := target.Profile, target.Season
	for i := range r.items {
		m := &r.items[i]
		if m.ID == id || (season != 0 && m.Profile == profile && m.Season == season) {
			m.Save = name
			if e := atomicJSON(filepath.Join(r.dir, m.ID+".json"), m); e != nil {
				return e
			}
		}
	}
	if m := r.current; m != nil && (m.ID == id || (season != 0 && m.Profile == profile && m.Season == season)) {
		m.Save = name
		return r.persist()
	}
	return nil
}
func readJournal(path string, accept func([]byte)) (bool, error) {
	return readJournalContext(context.Background(), path, false, accept)
}

// Analysis consumes packets synchronously and copies any pending frame. Its
// borrowed mode reuses one bounded packet buffer instead of allocating a new
// packet for every datagram. Other callers retain the original owned contract.
func readJournalContext(ctx context.Context, path string, borrow bool, accept func([]byte)) (bool, error) {
	f, e := os.Open(path)
	if e != nil {
		return false, e
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		return true, e
	}
	defer z.Close()
	var buffer [8192]byte
	for {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		var n [2]byte
		_, e = io.ReadFull(z, n[:])
		if e == io.EOF {
			return false, nil
		}
		if e != nil {
			return true, nil
		}
		size := int(binary.LittleEndian.Uint16(n[:]))
		if size < 29 || size > 8192 {
			return true, fmt.Errorf("invalid journal record size")
		}
		var b []byte
		if borrow {
			b = buffer[:size]
		} else {
			b = make([]byte, size)
		}
		if _, e = io.ReadFull(z, b); e != nil {
			return true, nil
		}
		accept(b)
	}
}
