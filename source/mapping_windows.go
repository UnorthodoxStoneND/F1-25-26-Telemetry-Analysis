package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var createSnapshot = kernel.NewProc("CreateToolhelp32Snapshot")
var processFirst = kernel.NewProc("Process32FirstW")
var processNext = kernel.NewProc("Process32NextW")
var openMapping = kernel.NewProc("OpenFileMappingW")
var mapView = kernel.NewProc("MapViewOfFile")
var unmapView = kernel.NewProc("UnmapViewOfFile")
var waitProcess = kernel.NewProc("WaitForSingleObject")
var readOwnMapping = kernel.NewProc("ReadProcessMemory")
var errUpdating = errors.New("Live snapshot is updating.")

// Copies only our own read-only mapped view. The game handle is SYNCHRONIZE-only.
func copyMapping(at uintptr, to []byte) bool {
	var got uintptr
	ok, _, _ := readOwnMapping.Call(^uintptr(0), at, uintptr(unsafe.Pointer(&to[0])), uintptr(len(to)), uintptr(unsafe.Pointer(&got)))
	return ok != 0 && got == uintptr(len(to))
}

type processEntry struct {
	Size, Usage, PID        uint32
	Heap                    uintptr
	Module, Threads, Parent uint32
	Priority                int32
	Flags                   uint32
	Exe                     [260]uint16
}
type Reader struct {
	mapping, process syscall.Handle
	address          uintptr
	pid              uint32
	buffer           [frameSize]byte
	cars             [maxCars]Car
}

func (r *Reader) Close() {
	if r.address != 0 {
		unmapView.Call(r.address)
	}
	if r.mapping != 0 {
		syscall.CloseHandle(r.mapping)
	}
	if r.process != 0 {
		syscall.CloseHandle(r.process)
	}
	r.address = 0
	r.mapping = 0
	r.process = 0
	r.pid = 0
	r.cars = [maxCars]Car{}
}
func (r *Reader) attach() bool {
	h, _, _ := createSnapshot.Call(2, 0)
	if h == ^uintptr(0) {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	e := processEntry{}
	e.Size = uint32(unsafe.Sizeof(e))
	ok, _, _ := processFirst.Call(h, uintptr(unsafe.Pointer(&e)))
	for ok != 0 {
		if strings.EqualFold(syscall.UTF16ToString(e.Exe[:]), "F1_25.exe") {
			// SYNCHRONIZE only: never open another process for memory reads or writes.
			proc, err := syscall.OpenProcess(0x100000, false, e.PID)
			if err == nil {
				name, _ := syscall.UTF16PtrFromString(fmt.Sprintf("Local\\USTelemetryLive-1-%d", e.PID))
				m, _, _ := openMapping.Call(4, 0, uintptr(unsafe.Pointer(name)))
				if m != 0 {
					address, _, _ := mapView.Call(m, 4, 0, 0, frameSize)
					if address != 0 {
						r.mapping = syscall.Handle(m)
						r.process = proc
						r.address = address
						r.pid = e.PID
						return true
					}
					syscall.CloseHandle(syscall.Handle(m))
				}
				syscall.CloseHandle(proc)
			}
		}
		ok, _, _ = processNext.Call(h, uintptr(unsafe.Pointer(&e)))
	}
	return false
}
func (r *Reader) Read() (Frame, error) {
	if r.address == 0 && !r.attach() {
		return Frame{Player: -1}, fmt.Errorf("Waiting for a game launched with this add-on enabled.")
	}
	state, _, _ := waitProcess.Call(uintptr(r.process), 0)
	if state != 258 {
		r.Close()
		return Frame{Player: -1}, fmt.Errorf("Game stopped.")
	}
	// One writer, two sequence checks. Never display a partly copied snapshot.
	var sequence [8]byte
	for attempt := 0; attempt < 4; attempt++ {
		if !copyMapping(r.address+16, sequence[:]) {
			break
		}
		before := binary.LittleEndian.Uint64(sequence[:])
		if before&1 != 0 {
			continue
		}
		if !copyMapping(r.address, r.buffer[:]) || !copyMapping(r.address+16, sequence[:]) {
			break
		}
		after := binary.LittleEndian.Uint64(sequence[:])
		if before == after && after&1 == 0 && binary.LittleEndian.Uint64(r.buffer[16:]) == after {
			return decodeInto(r.buffer[:], r.cars[:0])
		}
	}
	return Frame{}, errUpdating
}
