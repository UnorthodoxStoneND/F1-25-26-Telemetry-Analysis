package main

import (
	"encoding/binary"
	"fmt"
)

func fixture() []byte {
	b := make([]byte, frameSize)
	copy(b, "USTLIVE2")
	binary.LittleEndian.PutUint32(b[8:], 2)
	binary.LittleEndian.PutUint32(b[12:], frameSize)
	binary.LittleEndian.PutUint32(b[40:], active)
	binary.LittleEndian.PutUint32(b[48:], 1)
	copy(b[256+140:], "Test Driver")
	return b
}
func grid() Frame {
	f := Frame{Player: 12, State: active, Session: 1}
	for n := 0; n < 24; n++ {
		f.Cars = append(f.Cars, Car{Vehicle: uint32(n), Known: knownName | knownPosition | knownTyre, Position: int32(n + 1), Name: fmt.Sprintf("Driver %d", n+1), Compound: 18, Visual: 17, Identity: uint64(100 + n)})
	}
	return f
}
