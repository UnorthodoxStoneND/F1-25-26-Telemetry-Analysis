package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"sort"
	"strings"
	"time"
)

type Car struct {
	Name                             string
	Team                             int
	AI, MyTeam, Public, Participant  bool
	Position, Grid, Lap, Result, Pit int
	Last, Best                       uint32
	Compound, Visual, Age            int
	Wear                             [4]float64 // UDP order: RL, RR, FL, FR
	HasLap, HasStatus, HasWear       bool
	Setup                            []byte
}

type Telemetry struct {
	UID                                   uint64
	Format, Player, Count, Session, Track int
	Active, Ended, Paused                 bool
	Last                                  time.Time
	Cars                                  [24]Car
	frames                                map[byte]uint32
	retired                               map[uint64]bool
	lapClock                              float64
	historyFloor                          uint32
	Started                               bool
}

func u16(b []byte) int      { return int(binary.LittleEndian.Uint16(b)) }
func u32(b []byte) uint32   { return binary.LittleEndian.Uint32(b) }
func f32(b []byte) float64  { return float64(math.Float32frombits(u32(b))) }
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (t *Telemetry) reset(uid uint64, format int) {
	old := t.retired
	if old == nil {
		old = map[uint64]bool{}
	}
	if t.UID != 0 && t.UID != uid {
		old[t.UID] = true
	}
	if len(old) > 128 {
		old = map[uint64]bool{t.UID: true}
	}
	*t = Telemetry{UID: uid, Format: format, Player: -1, Track: -1, frames: map[byte]uint32{}, retired: old}
}

func (t *Telemetry) stop() {
	t.Cars = [24]Car{}
	t.Count, t.Active, t.Ended, t.Paused = 0, false, true, false
}

// Decode only documented 2025 (22 slots) and 2026 (24 slots) layouts.
// Validate the whole array before touching any session state.
func (t *Telemetry) ingest(b []byte) bool {
	if len(b) < 29 || b[5] != 1 {
		return false
	}
	format, id := u16(b), b[6]
	if format != 2025 && format != 2026 {
		return false
	}
	n := 22
	if format == 2026 {
		n = 24
	}
	stride, prefix, trailer := 0, 0, 0
	switch id {
	case 1:
		if len(b) < 48 {
			return false
		}
	case 2:
		stride, trailer = 57, 2
	case 3:
		if len(b) < 33 {
			return false
		}
	case 4:
		stride, prefix = 57, 1
		if format == 2026 {
			stride = 60
		}
	case 5:
		stride, trailer = 50, 4
	case 7:
		stride = 55
		if format == 2026 {
			stride = 59
		}
	case 10:
		stride = 46
	case 11:
		if len(b) != 1460 || int(b[29]) >= n || b[30] > 100 || b[31] > 8 {
			return false
		}
	default:
		return false
	}
	if stride != 0 && len(b) != 29+prefix+n*stride+trailer {
		return false
	}
	uid := binary.LittleEndian.Uint64(b[7:])
	// Lifecycle events are not ordinary frame samples. Several different events
	// can share a frozen frame, and shutdown may reset its counters / UID to 0.
	// Never let an earlier button event suppress SEND or SSTA.
	if id == 3 {
		code := string(b[29:33])
		if code != "SEND" && code != "SSTA" {
			return false
		}
		if code == "SEND" {
			if t.frames == nil || format != t.Format || (uid != 0 && uid != t.UID) {
				return false
			}
			t.stop()
			return true
		}
		if t.retired[uid] {
			return false
		}
		frame := u32(b[23:])
		if uid == t.UID && format == t.Format && t.Started && !t.Ended {
			if previous, ok := t.frames[3]; ok && previous == frame {
				return false
			}
		}
		if t.frames == nil || uid != t.UID || format != t.Format || t.Started || t.Ended {
			t.reset(uid, format)
		}
		t.Started, t.Active, t.Last = true, true, time.Now()
		t.frames[3] = frame
		if int(b[27]) < n {
			t.Player = int(b[27])
		}
		return true
	}
	// An explicit menu Session packet is additional end evidence, not silence.
	// Require an established session so unknown startup metadata is harmless.
	if id == 1 && uid == 0 && t.Active && t.Session > 0 && b[35] == 0 && int8(b[36]) < 0 {
		t.stop()
		return true
	}
	if t.Ended && uid == 0 {
		return false
	}
	if t.retired[uid] {
		return false
	}
	if t.UID != uid || t.Format != format || t.frames == nil {
		t.reset(uid, format)
	}
	frame := u32(b[23:]) // overall frame counter does not rewind on flashback
	if id == 11 && int32(frame-t.historyFloor) < 0 {
		return false
	}
	// History rotates through cars, so each car needs its own sequence.
	key := id
	if id == 11 {
		key = 32 + b[29]
	}
	if prev, ok := t.frames[key]; ok && int32(frame-prev) < 0 {
		return false
	}
	if t.Ended {
		return false
	}
	t.frames[key] = frame
	if int(b[27]) < n {
		t.Player = int(b[27])
	}
	t.Last, t.Active = time.Now(), true
	p := b[29:]
	switch id {
	case 1:
		// Unknown loading/grid metadata does not establish a session end.
		if p[6] != 0 {
			t.Session = int(p[6])
		}
		if int8(p[7]) >= 0 {
			t.Track = int(int8(p[7]))
		}
		t.Paused = p[14] == 1
	case 4:
		t.Count = min(int(p[0]), n)
		for i := 0; i < n; i++ {
			r := p[1+i*stride : 1+(i+1)*stride]
			c := &t.Cars[i]
			name, team, my, privacy := 7, int(r[3]), 4, 39
			if format == 2026 {
				name, team, my, privacy = 10, u16(r[5:]), 7, 42
			}
			c.Name = strings.TrimSpace(string(bytes.SplitN(r[name:name+32], []byte{0}, 2)[0]))
			c.Team, c.AI, c.MyTeam, c.Public, c.Participant = team, r[0] == 1, r[my] == 1, r[privacy] == 1, i < t.Count
		}
	case 2:
		clock := f32(b[15:])
		if finite(clock) && clock+0.1 < t.lapClock {
			// A flashback invalidates cached future timing, including best laps.
			for i := range t.Cars {
				t.Cars[i].Best, t.Cars[i].Last = 0, 0
			}
			t.historyFloor = frame
		}
		t.lapClock = clock
		for i := 0; i < n; i++ {
			r, c := p[i*stride:(i+1)*stride], &t.Cars[i]
			c.Last, c.Position, c.Lap = u32(r), int(r[32]), int(r[33])
			c.Pit, c.Result, c.HasLap = int(r[34]), int(r[45]), true
			c.Grid = int(r[43])
		}
	case 7:
		for i := 0; i < n; i++ {
			r, c := p[i*stride:(i+1)*stride], &t.Cars[i]
			c.Compound, c.Visual, c.Age, c.HasStatus = int(r[25]), int(r[26]), int(r[27]), true
		}
	case 10:
		for i := 0; i < n; i++ {
			r, c := p[i*stride:(i+1)*stride], &t.Cars[i]
			c.HasWear = true
			for w := range c.Wear {
				v := f32(r[w*4:])
				c.Wear[w] = v
				if !finite(v) || v < 0 || v > 100 {
					c.HasWear = false
				}
			}
		}
	case 5:
		for i := 0; i < n; i++ {
			r := p[i*stride : (i+1)*stride]
			// All-zero setups denote absent data, never a real all-zero setup.
			if !bytes.Equal(r, make([]byte, 50)) {
				t.Cars[i].Setup = bytes.Clone(r)
			} else {
				t.Cars[i].Setup = nil
			}
		}
	case 11:
		c := &t.Cars[p[0]]
		c.Best = 0
		for lap := 0; lap < int(p[1]); lap++ {
			r := p[7+14*lap : 7+14*(lap+1)]
			ms := u32(r)
			if r[13]&1 != 0 && ms > 0 && (c.Best == 0 || ms < c.Best) {
				c.Best = ms
			}
		}
	}
	return true
}

func (t *Telemetry) visible(mode string) []int {
	ids := []int{}
	if mode == "none" || !t.Active {
		return ids
	}
	for i, c := range t.Cars {
		if !c.Participant {
			continue
		}
		ids = append(ids, i)
	}
	sort.SliceStable(ids, func(a, b int) bool {
		x, y := position(t.Cars[ids[a]]), position(t.Cars[ids[b]])
		if x == 0 {
			x = 999
		}
		if y == 0 {
			y = 999
		}
		return x < y
	})
	if mode != "nearest" {
		return ids
	}
	idx := -1
	for k, i := range ids {
		if i == t.Player {
			idx = k
		}
	}
	if idx < 0 {
		// Show received participants while the game has not identified the
		// player's slot yet, instead of hiding the entire starting grid.
		return ids
	}
	start := max(0, min(idx-2, len(ids)-5))
	return ids[start:min(start+5, len(ids))]
}

func lapTime(ms uint32) string {
	if ms == 0 {
		return "—"
	}
	return fmtTime(ms)
}
