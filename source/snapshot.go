package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const analysisOffset = 10752
const analysisCarOffset = analysisOffset + 216
const analysisCarSize = 96
const frameSize = analysisCarOffset + analysisCarSize*maxCars
const carSize = 328
const maxCars = 32
const (
	active              = 1
	paused              = 2
	sessionEnded        = 32
	snapshotStale       = 64
	sessionTimer        = 128
	sessionLaps         = 256
	windowReady         = 512
	classificationOrder = 1024
)
const (
	knownName        = 1
	knownPosition    = 2
	knownGrid        = 4
	knownTyre        = 8
	knownLast        = 16
	knownBest        = 32
	knownPit         = 64
	knownResult      = 128
	knownLaps        = 256
	knownGapAhead    = 512
	knownSectors     = 1024
	knownTyreAge     = 2048
	knownLastSectors = 4096
	knownPenalties   = 8192
	knownTimePenalty = 16384
)
const inPitLane = 2
const outLap = 4
const resultMask = 7 << 8

type Car struct {
	Vehicle, Known                    uint32
	Position, Grid, Compound, Visual  int32
	Last, Best, WearKnown, SetupKnown uint32
	Wear                              [4]float32
	Setup                             [21]float32
	Name                              string
	BadgePNG                          string
	TyrePNG                           string
	Flags                             uint32
	Identity                          uint64
	Laps                              uint32
	TyreAgeLaps                       uint32
	GapAhead                          int32
	BestSectors                       [3]uint32
	LastSectors                       [3]uint32
	DriveThrough, StopGo              uint8
	TimePenaltyMS                     uint32
	Analysis                          AnalysisCar
}
type Frame struct {
	Session, Tick uint64
	State         uint32
	Player        int32
	SessionType   uint32
	SessionKind   uint32
	Progress      uint32
	Diagnostic    string
	Cars          []Car
	Analysis      AnalysisSession
}

func cstring(b []byte) string {
	if n := bytes.IndexByte(b, 0); n >= 0 {
		b = b[:n]
	}
	return strings.ToValidUTF8(string(b), "�")
}
func decode(b []byte) (Frame, error) {
	return decodeInto(b, nil)
}
func decodeInto(b []byte, storage []Car) (Frame, error) {
	f := Frame{Player: -1}
	if len(b) != frameSize || string(b[:8]) != "USTLIVE6" || binary.LittleEndian.Uint32(b[8:]) != 6 || binary.LittleEndian.Uint32(b[12:]) != frameSize {
		return f, fmt.Errorf("incompatible live snapshot")
	}
	u := func(n int) uint32 { return binary.LittleEndian.Uint32(b[n:]) }
	f.Session = binary.LittleEndian.Uint64(b[24:])
	f.Tick = binary.LittleEndian.Uint64(b[32:])
	f.State = u(40)
	f.Player = int32(u(44))
	f.SessionType = u(52)
	f.SessionKind = u(56)
	f.Progress = u(60)
	f.Diagnostic = cstring(b[64:256])
	at := analysisOffset
	u64 := func(n int) uint64 { return binary.LittleEndian.Uint64(b[n:]) }
	f.Analysis = AnalysisSession{
		Run: u64(at), Attempt: u64(at + 8), Sample: u64(at + 16),
		Season: int32(u(at + 24)), Round: int32(u(at + 28)), EndReason: u(at + 32),
		SafetyCarKnown: b[at+36] != 0, SafetyCar: b[at+37], Chequered: b[at+38] != 0,
		Track: cstring(b[at+40 : at+88]), Career: cstring(b[at+88 : at+216]),
	}
	count := u(48)
	if count > maxCars {
		return Frame{}, fmt.Errorf("invalid driver count")
	}
	f.Cars = storage[:0]
	seen := uint32(0)
	for n := 0; n < int(count); n++ {
		at := 256 + carSize*n
		c := Car{Vehicle: u(at), Known: u(at + 4), Position: int32(u(at + 8)), Grid: int32(u(at + 12)), Compound: int32(u(at + 16)), Visual: int32(u(at + 20)), Last: u(at + 24), Best: u(at + 28), WearKnown: u(at + 32), SetupKnown: u(at + 36), Name: cstring(b[at+140 : at+268]), Flags: u(at + 268), Identity: binary.LittleEndian.Uint64(b[at+272:])}
		if c.Vehicle >= maxCars || seen&(1<<c.Vehicle) != 0 {
			return Frame{}, fmt.Errorf("invalid vehicle identity")
		}
		seen |= 1 << c.Vehicle
		extra := analysisCarOffset + int(c.Vehicle)*analysisCarSize
		float := func(offset int) float32 { return math.Float32frombits(u(extra + offset)) }
		c.Analysis = AnalysisCar{
			Known: u(extra), Current: u(extra + 4), BestLap: u(extra + 8), CurrentS1: u(extra + 12), CurrentS2: u(extra + 16),
			GapLeader: int32(u(extra + 20)), Gear: int32(u(extra + 24)), Lap: int32(u(extra + 28)),
			Speed: float(32), Throttle: float(36), Brake: float(40), RPM: float(44), LapDistance: float(48),
			LapFlags: b[extra+52], PitStatus: b[extra+53], PitStops: b[extra+54], Stint: b[extra+55],
			Team: cstring(b[extra+56 : extra+96]),
		}
		c.DriveThrough, c.StopGo = b[at+316], b[at+317]
		c.TimePenaltyMS = u(at + 320)
		if c.TimePenaltyMS > 3600000 {
			c.Known &^= knownTimePenalty
		}
		c.TyreAgeLaps = u(at + 300)
		if c.TyreAgeLaps > 255 {
			c.Known &^= knownTyreAge
		}
		c.Laps = u(at + 280)
		c.GapAhead = int32(u(at + 284))
		for i := range c.BestSectors {
			c.BestSectors[i] = u(at + 288 + 4*i)
		}
		for i := range c.LastSectors {
			c.LastSectors[i] = u(at + 304 + 4*i)
		}
		if c.Laps >= 1000 {
			c.Known &^= knownLaps
		}
		if c.GapAhead < 0 || c.GapAhead > 10800000 {
			c.Known &^= knownGapAhead
		}
		var total uint64
		for _, ms := range c.BestSectors {
			if ms == 0 || ms >= 3600000 {
				c.Known &^= knownSectors
			}
			total += uint64(ms)
		}
		difference := int64(total) - int64(c.Best)
		if c.Known&knownBest == 0 || c.Best == 0 || difference < -10 || difference > 10 {
			c.Known &^= knownSectors
		}
		var lastTotal uint64
		for _, ms := range c.LastSectors {
			if ms == 0 || ms >= 3600000 {
				c.Known &^= knownLastSectors
			}
			lastTotal += uint64(ms)
		}
		lastDifference := int64(lastTotal) - int64(c.Last)
		if c.Known&knownLast == 0 || c.Last == 0 || lastDifference < -10 || lastDifference > 10 {
			c.Known &^= knownLastSectors
		}
		for i := range c.Wear {
			c.Wear[i] = math.Float32frombits(u(at + 40 + 4*i))
			if !finite(c.Wear[i]) || c.Wear[i] < 0 || c.Wear[i] > 100 {
				c.WearKnown &^= 1 << i
			}
		}
		for i := range c.Setup {
			c.Setup[i] = math.Float32frombits(u(at + 56 + 4*i))
			if !finite(c.Setup[i]) {
				c.SetupKnown &^= 1 << i
			}
		}
		f.Cars = append(f.Cars, c)
	}
	if f.State&active == 0 {
		f.Cars = nil
	}
	return f, nil
}
func finite(f float32) bool { return !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0) }
