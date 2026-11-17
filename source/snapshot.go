package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const frameSize = 9216
const maxCars = 32
const (
	active      = 1
	paused      = 2
	apiReady    = 4
	directReady = 8
)
const (
	knownName     = 1
	knownPosition = 2
	knownGrid     = 4
	knownTyre     = 8
	knownLast     = 16
	knownBest     = 32
	knownPit      = 64
)
const inPitLane = 2

type Car struct {
	Vehicle, Known                    uint32
	Position, Grid, Compound, Visual  int32
	Last, Best, WearKnown, SetupKnown uint32
	Wear                              [4]float32
	Setup                             [21]float32
	Name                              string
	Flags                             uint32
	Identity                          uint64
}
type Frame struct {
	Session, Tick uint64
	State         uint32
	Player        int32
	SessionType   uint32
	Diagnostic    string
	Cars          []Car
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
	if len(b) != frameSize || string(b[:8]) != "USTLIVE1" || binary.LittleEndian.Uint32(b[8:]) != 1 || binary.LittleEndian.Uint32(b[12:]) != frameSize {
		return f, fmt.Errorf("incompatible live snapshot")
	}
	u := func(n int) uint32 { return binary.LittleEndian.Uint32(b[n:]) }
	f.Session = binary.LittleEndian.Uint64(b[24:])
	f.Tick = binary.LittleEndian.Uint64(b[32:])
	f.State = u(40)
	f.Player = int32(u(44))
	f.SessionType = u(52)
	f.Diagnostic = cstring(b[64:256])
	count := u(48)
	if count > maxCars {
		return Frame{}, fmt.Errorf("invalid driver count")
	}
	f.Cars = storage[:0]
	seen := uint32(0)
	for n := 0; n < int(count); n++ {
		at := 256 + 280*n
		c := Car{Vehicle: u(at), Known: u(at + 4), Position: int32(u(at + 8)), Grid: int32(u(at + 12)), Compound: int32(u(at + 16)), Visual: int32(u(at + 20)), Last: u(at + 24), Best: u(at + 28), WearKnown: u(at + 32), SetupKnown: u(at + 36), Name: cstring(b[at+140 : at+268]), Flags: u(at + 268), Identity: binary.LittleEndian.Uint64(b[at+272:])}
		if c.Vehicle >= maxCars || seen&(1<<c.Vehicle) != 0 {
			return Frame{}, fmt.Errorf("invalid vehicle identity")
		}
		seen |= 1 << c.Vehicle
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
