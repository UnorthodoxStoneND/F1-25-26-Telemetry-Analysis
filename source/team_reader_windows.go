package main

// External, read-only identity access. No injected code, game function calls,
// writes, scanning for player-specific text, or dependence on previous snapshots.
import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const verifiedTeamBuild = "d873a1c8d2044cef2fd76c99c9d5a97649c5b815d055540995105f46619865ae"

var teamKernel = syscall.NewLazyDLL("kernel32.dll")
var teamReadMemory = teamKernel.NewProc("ReadProcessMemory")

type teamModule struct {
	Size, ModuleID, ProcessID, GlobalCount, ProcessCount uint32
	Base                                                 uintptr
	ImageSize                                            uint32
	Module                                               syscall.Handle
	Name                                                 [256]uint16
	Path                                                 [260]uint16
}
type teamMemory struct{ handle syscall.Handle }

func (m teamMemory) read(addr uintptr, n int) []byte {
	if addr < 0x10000 || addr > 0x7fffffffffff || n <= 0 || n > 16*1024*1024 {
		return nil
	}
	b := make([]byte, n)
	var count uintptr
	ok, _, _ := teamReadMemory.Call(uintptr(m.handle), addr, uintptr(unsafe.Pointer(&b[0])), uintptr(n), uintptr(unsafe.Pointer(&count)))
	if ok == 0 || count != uintptr(n) {
		return nil
	}
	return b
}

type teamMemoryReader interface{ read(uintptr, int) []byte }

func teamPtr(m teamMemoryReader, addr uintptr) uintptr {
	b := m.read(addr, 8)
	if len(b) != 8 {
		return 0
	}
	return uintptr(binary.LittleEndian.Uint64(b))
}
func teamCString(m teamMemoryReader, addr uintptr, n int) string {
	b := m.read(addr, n)
	if len(b) != n {
		return ""
	}
	end := bytes.IndexByte(b, 0)
	if end < 0 {
		return ""
	}
	s := string(b[:end])
	if !validTeamText(s) {
		return ""
	}
	return s
}
func loadedTeamProcess() (uint32, error) {
	h, e := syscall.CreateToolhelp32Snapshot(2, 0)
	if e != nil {
		return 0, e
	}
	defer syscall.CloseHandle(h)
	p := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	var pid uint32
	for e = syscall.Process32First(h, &p); e == nil; e = syscall.Process32Next(h, &p) {
		if strings.EqualFold(syscall.UTF16ToString(p.ExeFile[:]), "F1_25.exe") {
			if pid != 0 {
				return 0, fmt.Errorf("Multiple game processes; team names cannot be matched safely.")
			}
			pid = p.ProcessID
		}
	}
	if pid == 0 {
		return 0, fmt.Errorf("The game was not available for team-name capture.")
	}
	return pid, nil
}
func loadedTeamModule(pid uint32) (teamModule, error) {
	h, e := syscall.CreateToolhelp32Snapshot(0x18, pid)
	if e != nil {
		return teamModule{}, e
	}
	defer syscall.CloseHandle(h)
	module := teamModule{Size: uint32(unsafe.Sizeof(teamModule{}))}
	first := teamKernel.NewProc("Module32FirstW")
	next := teamKernel.NewProc("Module32NextW")
	ok, _, _ := first.Call(uintptr(h), uintptr(unsafe.Pointer(&module)))
	for ok != 0 {
		if strings.EqualFold(syscall.UTF16ToString(module.Name[:]), "F1_25.exe") {
			return module, nil
		}
		ok, _, _ = next.Call(uintptr(h), uintptr(unsafe.Pointer(&module)))
	}
	return teamModule{}, fmt.Errorf("Cannot locate the game module for team-name capture.")
}
func captureLoadedTeams(launcherData string) (*TeamSnapshot, error) {
	pid, e := loadedTeamProcess()
	if e != nil {
		return nil, e
	}
	module, e := loadedTeamModule(pid)
	if e != nil {
		return nil, e
	}
	h, e := syscall.OpenProcess(0x410, false, pid)
	if e != nil {
		return nil, fmt.Errorf("Read-only team-name access is unavailable: %w", e)
	}
	defer syscall.CloseHandle(h)
	var created, exited, kernel, user syscall.Filetime
	if e = syscall.GetProcessTimes(h, &created, &exited, &kernel, &user); e != nil {
		return nil, e
	}
	path := syscall.UTF16ToString(module.Path[:])
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	hash := sha256.New()
	_, e = io.Copy(hash, f)
	f.Close()
	if e != nil {
		return nil, e
	}
	build := fmt.Sprintf("%x", hash.Sum(nil))
	if build != verifiedTeamBuild {
		return nil, fmt.Errorf("Team-name capture does not support this game build; UDP recording continues.")
	}
	m := teamMemory{h}
	base := module.Base
	dos := m.read(base, 64)
	if len(dos) != 64 || string(dos[:2]) != "MZ" {
		return nil, fmt.Errorf("Cannot verify loaded game image.")
	}
	pe := m.read(base+uintptr(binary.LittleEndian.Uint32(dos[60:])), 12)
	if len(pe) != 12 || string(pe[:4]) != "PE\x00\x00" || binary.LittleEndian.Uint32(pe[8:]) != 1789118616 {
		return nil, fmt.Errorf("Loaded game build is not verified for team-name capture.")
	}
	manager := teamPtr(m, base+0x604c2c8)
	context := teamPtr(m, manager+0xc0)
	mode := teamPtr(m, context+0x88)
	system := teamPtr(m, manager+0x290)
	modeName := teamCString(m, teamPtr(m, mode+0x278), 32)
	if modeName == "" || system == 0 {
		return nil, fmt.Errorf("The game's active team records are not ready yet.")
	}
	teams, e := readLoadedTeamTables(m, base, system)
	if e != nil {
		return nil, e
	}
	root := filepath.Dir(filepath.Dir(launcherData))
	game := filepath.Dir(path)
	texts, language, warnings := loadTeamLanguage(root, game, modeName, time.Unix(0, created.Nanoseconds()))
	for i := range teams {
		teams[i].Name = resolveTeamText(teams[i].RawName, texts)
		teams[i].ShortName = resolveTeamText(teams[i].ShortName, texts)
		teams[i].Code = resolveTeamText(teams[i].Code, texts)
	}
	// Reject a snapshot if the game unloaded/switched mode during the read.
	if teamPtr(m, base+0x604c2c8) != manager || teamPtr(m, manager+0x290) != system || teamPtr(m, context+0x88) != mode {
		return nil, fmt.Errorf("The loaded session changed during team-name capture; retrying.")
	}
	snapshot := &TeamSnapshot{Captured: time.Now().UTC(), PID: pid, ProcessStarted: time.Unix(0, created.Nanoseconds()).UTC(), Build: build, Mode: modeName, Language: language, Teams: teams, Warnings: warnings}
	snapshot.Entrants = readLoadedTeamEntrants(m, base, teamPtr(m, mode+0x260))
	loadTeamRewrites(filepath.Join(game, "swofty", "udp_ids.json"), snapshot)
	return snapshot, nil
}
func readLoadedTeamEntrants(m teamMemoryReader, base, manager uintptr) []CapturedEntrant {
	begin, end := teamPtr(m, manager+0x2e0), teamPtr(m, manager+0x2e8)
	if begin == 0 || end < begin || (end-begin)%0x88 != 0 || (end-begin)/0x88 > 512 {
		return nil
	}
	b := m.read(begin, int(end-begin))
	out := []CapturedEntrant{}
	for off := 0; off+0x88 <= len(b); off += 0x88 {
		r := b[off : off+0x88]
		if uintptr(binary.LittleEndian.Uint64(r)) != base+0x5612e20 || r[0x82] == 255 {
			continue
		}
		team := binary.LittleEndian.Uint32(r[0x58:])
		if team&255 > 2 || team>>16 == 0x3fff {
			continue
		}
		out = append(out, CapturedEntrant{DriverID: int(binary.LittleEndian.Uint16(r[0x52:])), Number: int(r[0x83]), Set: int(team & 255), TeamID: int(team >> 16)})
	}
	return out
}

// Layout evidence: SwoftyLauncher re/db/structs/{TeamRecord,TeamSlots,
// TeamSlot,TeamSetRef,EntrantsSystem}.h on the verified Steam build.
func readLoadedTeamTables(m teamMemoryReader, base, system uintptr) ([]CapturedTeam, error) {
	teams := []CapturedTeam{}
	for set := 0; set < 3; set++ {
		slots := teamPtr(m, system+0x2c8+uintptr(set)*16)
		if slots == 0 {
			continue
		}
		begin, end := teamPtr(m, slots+8), teamPtr(m, slots+16)
		if begin == end {
			continue
		}
		if begin == 0 || end < begin || (end-begin)%16 != 0 || (end-begin)/16 > 65536 {
			return nil, fmt.Errorf("Invalid loaded team table; refusing an unsafe read.")
		}
		entries := m.read(begin, int(end-begin))
		if len(entries) != int(end-begin) {
			return nil, fmt.Errorf("The loaded team table is not readable yet.")
		}
		for off := 0; off < len(entries); off += 16 {
			record := uintptr(binary.LittleEndian.Uint64(entries[off+8:]))
			if record == 0 {
				continue
			}
			b := m.read(record, 0x298)
			if len(b) != 0x298 {
				return nil, fmt.Errorf("A loaded team record is not readable yet.")
			}
			key := binary.LittleEndian.Uint32(b)
			if int(key&0xff) != set || int(key>>16) != off/16 {
				return nil, fmt.Errorf("Loaded team identity validation failed.")
			}
			field := func(offset int) string {
				if uintptr(binary.LittleEndian.Uint64(b[offset-16:])) != base+0x5613670 || binary.LittleEndian.Uint64(b[offset-8:]) != 128 {
					return ""
				}
				return teamCString(m, uintptr(binary.LittleEndian.Uint64(b[offset:])), 128)
			}
			label := field(0x18)
			if label == "" {
				return nil, fmt.Errorf("Loaded team string layout validation failed.")
			}
			teams = append(teams, CapturedTeam{Set: set, ID: off / 16, Label: label, RawName: field(0xb0), ShortName: field(0x1e0), Code: field(0x148)})
		}
	}
	if len(teams) == 0 {
		return nil, fmt.Errorf("No loaded team names are available yet.")
	}
	return teams, nil
}
