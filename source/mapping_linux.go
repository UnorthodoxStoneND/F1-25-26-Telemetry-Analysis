package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var errUpdating = errors.New("Live snapshot is updating.")

const protonFrameSize = frameSize + badgeFrameSize

// Read only the add-on's temporary IPC mapping, never /proc game memory. The
// launcher supplies the actual game-copy add-on directory through its host API.
type Reader struct {
	addonDir    string
	file        *os.File
	identity    os.FileInfo
	mapped      []byte
	address     uintptr
	buffer      [frameSize]byte
	badgeBuffer []byte
	cars        [maxCars]Car
	badges      badgeCache
}

func (r *Reader) setAddonDir(path string) {
	if path != r.addonDir {
		r.Close()
		r.addonDir = path
	}
}
func (r *Reader) Close() {
	if r.mapped != nil {
		_ = syscall.Munmap(r.mapped)
	}
	if r.file != nil {
		_ = r.file.Close()
	}
	r.file = nil
	r.mapped = nil
	r.identity = nil
	r.address = 0
	r.badgeBuffer = nil
	r.badges = badgeCache{}
	r.cars = [maxCars]Car{}
}
func (r *Reader) attach() bool {
	if !filepath.IsAbs(r.addonDir) {
		return false
	}
	paths, _ := filepath.Glob(filepath.Join(r.addonDir, "bin", ".telemetry-live-*.ipc"))
	// Prefer the newest game; cap discovery even if a prior crash left junk.
	type candidate struct {
		path string
		info os.FileInfo
	}
	files := []candidate{}
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err == nil && info.Mode().IsRegular() && info.Size() == protonFrameSize {
			files = append(files, candidate{path, info})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].info.ModTime().After(files[j].info.ModTime()) })
	if len(files) > 32 {
		files = files[:32]
	}
	for _, item := range files {
		fd, err := syscall.Open(item.path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
		if err != nil {
			continue
		}
		file := os.NewFile(uintptr(fd), item.path)
		info, err := file.Stat()
		if err != nil || info.Size() != protonFrameSize || !os.SameFile(info, item.info) {
			file.Close()
			continue
		}
		mapped, err := syscall.Mmap(fd, 0, protonFrameSize, syscall.PROT_READ, syscall.MAP_SHARED)
		if err != nil {
			file.Close()
			continue
		}
		if string(mapped[:8]) != "USTLIVE5" || binary.LittleEndian.Uint32(mapped[12:]) != frameSize {
			syscall.Munmap(mapped)
			file.Close()
			continue
		}
		r.file = file
		r.identity = info
		r.mapped = mapped
		r.address = 1
		r.badgeBuffer = make([]byte, badgeFrameSize)
		return true
	}
	return false
}
func (r *Reader) Read() (Frame, error) {
	if r.mapped == nil && !r.attach() {
		return Frame{Player: -1}, fmt.Errorf("Waiting for a game launched through Proton with this add-on enabled.")
	}
	info, err := os.Lstat(r.file.Name())
	if err != nil || !os.SameFile(info, r.identity) || info.Size() != protonFrameSize {
		r.Close()
		return Frame{Player: -1}, fmt.Errorf("Game stopped.")
	}
	sequence := func(offset int) uint64 { return atomic.LoadUint64((*uint64)(unsafe.Pointer(&r.mapped[offset]))) }
	for attempt := 0; attempt < 4; attempt++ {
		before := sequence(16)
		if before&1 != 0 {
			continue
		}
		copy(r.buffer[:], r.mapped[:frameSize])
		if binary.LittleEndian.Uint64(r.buffer[16:]) != before {
			continue
		}
		activeFrame := binary.LittleEndian.Uint32(r.buffer[40:])&active != 0
		if activeFrame {
			if sequence(frameSize+16) != before {
				continue
			}
			copy(r.badgeBuffer, r.mapped[frameSize:])
			if sequence(frameSize+16) != before || binary.LittleEndian.Uint64(r.badgeBuffer[16:]) != before {
				continue
			}
		}
		if sequence(16) != before {
			continue
		}
		frame, err := decodeInto(r.buffer[:], r.cars[:0])
		if err == nil {
			if activeFrame {
				r.badges.apply(r.badgeBuffer, &frame)
			} else {
				r.badges = badgeCache{}
			}
		}
		return frame, err
	}
	return Frame{}, errUpdating
}
