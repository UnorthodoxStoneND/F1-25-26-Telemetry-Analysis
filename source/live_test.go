//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestSnapshotValidation(t *testing.T) {
	b := fixture()
	binary.LittleEndian.PutUint32(b[256+4:], knownName)
	binary.LittleEndian.PutUint32(b[256+32:], 15)
	binary.LittleEndian.PutUint32(b[256+40:], math.Float32bits(float32(math.NaN())))
	f, err := decode(b)
	if err != nil || len(f.Cars) != 1 || f.Cars[0].Name != "Test Driver" || f.Cars[0].WearKnown != 14 {
		t.Fatalf("bad decode: %+v %v", f, err)
	}
	for _, mutate := range []func([]byte){func(b []byte) { b[0] = 'X' }, func(b []byte) { b[8] = 1 }, func(b []byte) { b[48] = 33 }, func(b []byte) { b[256] = 32 }, func(b []byte) { b[48] = 2 }} {
		x := fixture()
		mutate(x)
		if _, err = decode(x); err == nil {
			t.Fatal("accepted malformed snapshot")
		}
	}
	b = fixture()
	b[40] = 0
	f, err = decode(b)
	if err != nil || len(f.Cars) != 0 {
		t.Fatal("ended session retained cars")
	}
}
func TestNativeABI(t *testing.T) {
	path := os.Getenv("TELEMETRY_ABI_FRAME")
	if path == "" {
		t.Skip("native build runs this independently with its generated fixture")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := decode(b)
	if err != nil || f.Tick != 12345 || len(f.Cars) != 1 || f.Cars[0].Name != "ABI Driver" || f.Cars[0].Identity != 1234 || f.Cars[0].Compound != 18 || f.Cars[0].Visual != 17 || f.SessionType != 15 || f.SessionKind != 4 || f.Cars[0].Laps != 7 || f.Cars[0].TyreAgeLaps != 3 || f.Cars[0].Known&knownTyreAge == 0 || f.Cars[0].GapAhead != 1234 || f.Cars[0].BestSectors != [3]uint32{20000, 30000, 30000} || f.Cars[0].Known&knownLastSectors == 0 || f.Cars[0].LastSectors != [3]uint32{21000, 30000, 30000} || f.Cars[0].Known&knownPenalties == 0 || f.Cars[0].DriveThrough != 2 || f.Cars[0].StopGo != 3 || f.Cars[0].Known&knownTimePenalty == 0 || f.Cars[0].TimePenaltyMS != 5250 {
		t.Fatalf("C++/Go layout disagrees: %+v %v", f, err)
	}
}

func TestNearestCars(t *testing.T) {
	f := grid()
	for _, tt := range []struct {
		player int32
		start  uint32
	}{{0, 0}, {1, 0}, {12, 10}, {22, 19}, {23, 19}} {
		f.Player = tt.player
		c := visible(f, "nearest")
		if len(c) != 5 || c[0].Vehicle != tt.start {
			t.Fatalf("window player %d: %+v", tt.player, c)
		}
	}
	f.Player = -1
	if len(visible(f, "nearest")) != 0 || len(visible(f, "full")) != 24 {
		t.Fatal("unknown player guessed")
	}
}
func TestTyreImageCell(t *testing.T) {
	c := badgeFrame().Cars[0]
	c.TyrePNG = testBadgePNG(t)
	raw, _ := json.Marshal(tyreCell(c))
	if !bytes.Contains(raw, []byte("image_data")) || bytes.Contains(raw, []byte("colour")) {
		t.Fatalf("not an image cell: %s", raw)
	}
	c.TyrePNG = ""
	if tyreCell(c) != "—" {
		t.Fatal("missing image fabricated a compound")
	}
}

func TestLiveViewAndSetup(t *testing.T) {
	a := App{frame: grid(), enabled: true, mode: "full", status: "Live", highlight: true}
	a.frame.Cars[12].WearKnown = 7
	a.expanded = rowID(a.frame, a.frame.Cars[12])
	v := a.view()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`"image"`, `"chart"`, `"selection"`, `"display"`, `"YOU"`, `udp_port`, `Drivers shown`, `Highlight my row`, `Telemetry Analysis`} {
		if bytes.Contains(b, []byte(bad)) {
			t.Fatalf("unexpected live control: %s", bad)
		}
	}
	blocks := v["body"].(M)["blocks"].([]M)
	if len(blocks) != 3 || blocks[0]["table"] == nil || blocks[1]["card"] == nil || blocks[2]["table"] == nil {
		t.Fatal("setup was not inserted beneath its driver")
	}
	rows := blocks[0]["table"].(M)["rows"].([]M)
	r := rows[len(rows)-1]
	if r["tone"] != "accent" {
		t.Fatal("player highlight missing")
	}
	if path := os.Getenv("TELEMETRY_NATIVE_VIEWS"); path != "" {
		file, e := os.Create(path)
		if e != nil {
			t.Fatal(e)
		}
		defer file.Close()
		enc := json.NewEncoder(file)
		for i := range a.frame.Cars {
			c := &a.frame.Cars[i]
			c.Known |= knownLaps | knownGapAhead | knownGrid | knownBest | knownSectors | knownLast | knownLastSectors | knownResult | knownTyreAge | knownPenalties | knownTimePenalty
			c.Laps, c.GapAhead, c.Grid = 7, 1234, int32(i+1)
			c.TyreAgeLaps = 3
			c.Best, c.BestSectors = 80000, [3]uint32{20000, 30000, 30000}
			c.Last, c.LastSectors = 81000, [3]uint32{21000, 30000, 30000}
			c.DriveThrough, c.StopGo = 2, 1
			c.TimePenaltyMS = 5000
			c.Flags = 2 << 8
		}
		v = a.view()
		if e = enc.Encode(v); e != nil {
			t.Fatal(e)
		}
		a.frame.Cars[12].BadgePNG = testBadgePNG(t)
		a.frame.Cars[12].TyrePNG = testBadgePNG(t)
		if e = enc.Encode(a.view()); e != nil {
			t.Fatal(e)
		}
		for _, session := range []uint32{5, 10, 15, 16} {
			a.frame.SessionType = session
			a.expanded = ""
			if e = enc.Encode(a.view()); e != nil {
				t.Fatal(e)
			}
		}
		for _, session := range []uint32{1, 5, 15} {
			a.frame.SessionType = session
			layout := layoutFor(a.frame)
			a.columnsOpen = true
			a.editorLayout = layout
			if e = enc.Encode(a.view()); e != nil {
				t.Fatal(e)
			}
			a.columnsOpen = false
			a.hidden[layout] = fieldBit(layout, "position") | fieldBit(layout, "last")
			if e = enc.Encode(a.view()); e != nil {
				t.Fatal(e)
			}
			a.disabled[layout] = true
			if e = enc.Encode(a.view()); e != nil {
				t.Fatal(e)
			}
			a.disabled[layout] = false
			a.hidden[layout] = 0
		}
		a.frame = Frame{Player: -1}
		a.expanded = ""
		if e = enc.Encode(a.view()); e != nil {
			t.Fatal(e)
		}
		a.displayMode = "window"
		if e = enc.Encode(a.view()); e != nil {
			t.Fatal(e)
		}
	}
}
func TestPressureSetupForPlayerAndOtherCars(t *testing.T) {
	a := App{frame: grid(), enabled: true, mode: "full", status: "Live"}
	for _, driver := range []int{0, 12, 23} {
		c := &a.frame.Cars[driver]
		c.SetupKnown = 15 << 16
		copy(c.Setup[16:20], []float32{24.3, 24.4, 22.1, 22.2})
		a.expanded = rowID(a.frame, *c)
		raw, err := json.Marshal(a.view())
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{"Front-left pressure", "Front-right pressure", "Rear-left pressure", "Rear-right pressure", "24.3 psi", "24.4 psi", "22.1 psi", "22.2 psi"} {
			if !bytes.Contains(raw, []byte(text)) {
				t.Fatalf("driver %d missing %q from setup card", driver, text)
			}
		}
		a.lastView, a.viewSent = a.viewState(), true
		c.Setup[18] = 22.5
		if !a.needsView() {
			t.Fatalf("driver %d rear-pressure change did not refresh setup", driver)
		}
		c.SetupKnown &^= 1 << 18
		if setupValue(*c, 18) != "—" || setupValue(*c, 19) != "22.2 psi" {
			t.Fatal("missing rear reading hid another corner or fabricated a value")
		}
	}
}

func TestStaleSetupEvent(t *testing.T) {
	a := App{frame: grid()}
	c := a.frame.Cars[0]
	old := rowID(a.frame, c)
	a.frame.Session++
	event := fmt.Sprintf(`{"event":{"submit":{"form":"drivers","row":%q,"values":{"setup":true}}}}`, old)
	_, err := a.event([]byte(event))
	if err != nil || a.expanded != "" {
		t.Fatal("stale session opened setup")
	}
}
func TestLauncherSettings(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "addons", addonID)
	dir := filepath.Join(root, "profiles")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	profile := `{"options":{"` + addonID + `":{"drivers_shown":"full","highlight_player":true}}}`
	if err := os.WriteFile(filepath.Join(dir, "main.json"), []byte(profile), 0600); err != nil {
		t.Fatal(err)
	}
	a := App{profileName: "main", data: data}
	a.syncOptions()
	if a.mode != "full" || !a.highlight {
		t.Fatal("launcher settings not read")
	}
	a.profileName = "../main"
	a.syncOptions()
	if a.mode != "nearest" || a.highlight {
		t.Fatal("unsafe profile path accepted")
	}
}
func TestProtocolAndNoDiskWrites(t *testing.T) {
	root := t.TempDir()
	hello := fmt.Sprintf(`{"id":1,"method":"hello","params":{"api":1,"launcher":"1.9.76","data_dir":%q,"profile":{"name":"default","enabled":true}}}`, filepath.Join(root, "addons", addonID))
	input := hello + "\n" + `{"id":2,"method":"view","params":{"section":"telemetry"}}` + "\n" + `{"id":3,"method":"view","params":{"section":"analysis"}}` + "\n"
	var output bytes.Buffer
	if err := run(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("unexpected protocol: %s", output.String())
	}
	var third M
	_ = json.Unmarshal([]byte(lines[2]), &third)
	if third["error"] == nil {
		t.Fatal("analysis still enabled")
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 0 {
		t.Fatal("live helper wrote data")
	}
}
func TestReleasedAndFutureLauncherHandshake(t *testing.T) {
	for _, launcher := range []string{"1.9.80", "1.10.0", "2.0.0"} {
		input := fmt.Sprintf(`{"id":1,"method":"hello","params":{"api":1,"launcher":%q,"data_dir":"unused","profile":{"enabled":false}}}`+"\n", launcher)
		var output bytes.Buffer
		if err := run(strings.NewReader(input), &output); err != nil || !bytes.Contains(output.Bytes(), []byte(`"api":1`)) {
			t.Fatalf("compatible launcher rejected: %s", launcher)
		}
	}
	var output bytes.Buffer
	_ = run(strings.NewReader(`{"id":1,"method":"hello","params":{"api":2,"data_dir":"unused"}}`+"\n"), &output)
	if !bytes.Contains(output.Bytes(), []byte(`"error"`)) {
		t.Fatal("unsupported protocol silently accepted")
	}
}
func TestSharedMemoryAndProcessLifecycle(t *testing.T) {
	r := Reader{}
	defer r.Close()
	name, _ := syscall.UTF16PtrFromString(fmt.Sprintf("Local\\TelemetryTest-%d", os.Getpid()))
	create := kernel.NewProc("CreateFileMappingW")
	h, _, err := create.Call(^uintptr(0), 0, 4, 0, frameSize, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	address, _, err := mapView.Call(h, 2, 0, 0, frameSize)
	if address == 0 {
		t.Fatal(err)
	}
	defer unmapView.Call(address)
	write := func(at uintptr, b []byte) {
		t.Helper()
		var got uintptr
		ok, _, err := kernel.NewProc("WriteProcessMemory").Call(^uintptr(0), at, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&got)))
		if ok == 0 || got != uintptr(len(b)) {
			t.Fatal(err)
		}
	}
	write(address, fixture())
	readHandle, _, _ := openMapping.Call(4, 0, uintptr(unsafe.Pointer(name)))
	r.mapping = syscall.Handle(readHandle)
	r.address, _, _ = mapView.Call(readHandle, 4, 0, 0, frameSize)
	r.process, err = syscall.OpenProcess(0x100000, false, uint32(os.Getpid()))
	if err != nil || r.address == 0 {
		t.Fatal(err)
	}
	f, e := r.Read()
	if e != nil || len(f.Cars) != 1 {
		t.Fatal("read-only mapping failed", e)
	}
	// The actual helper refresh path must not disguise a reader failure as waiting.
	bad := fixture()
	binary.LittleEndian.PutUint32(bad[40:], 0)
	copy(bad[64:256], "Game binding unavailable: vehicle table.")
	write(address, bad)
	a := App{reader: r, enabled: true}
	a.refresh()
	if a.status != "Live data unavailable" || a.frame.Diagnostic == "" {
		t.Fatal("binding failure hidden behind waiting status")
	}
	viewBytes, _ := json.Marshal(a.view())
	if !bytes.Contains(viewBytes, []byte("Game binding unavailable")) {
		t.Fatal("diagnostic missing from live page")
	}
	write(address, fixture())
	sequence := make([]byte, 8)
	sequence[0] = 1
	write(address+16, sequence)
	if _, e = r.Read(); e == nil {
		t.Fatal("accepted in-progress copy")
	}
	sequence[0] = 2
	write(address+16, sequence)
	// A signalled event stands in for an exited process at WaitForSingleObject.
	syscall.CloseHandle(r.process)
	createEvent := kernel.NewProc("CreateEventW")
	event, _, _ := createEvent.Call(0, 1, 1, 0)
	r.process = syscall.Handle(event)
	_, e = r.Read()
	if e == nil || r.address != 0 || r.mapping != 0 || r.process != 0 {
		t.Fatal("exit did not release mapping and clear status")
	}
}
func TestBoundedLiveMemory(t *testing.T) {
	b := fixture()
	binary.LittleEndian.PutUint32(b[48:], 24)
	for id := 0; id < 24; id++ {
		at := 256 + carSize*id
		binary.LittleEndian.PutUint32(b[at:], uint32(id))
		binary.LittleEndian.PutUint32(b[at+4:], knownName|knownPosition|knownTyre)
		binary.LittleEndian.PutUint32(b[at+8:], uint32(id+1))
		binary.LittleEndian.PutUint32(b[at+16:], 18)
		binary.LittleEndian.PutUint32(b[at+20:], 17)
		copy(b[at+140:], fmt.Sprintf("Driver %d", id+1))
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	a := App{mode: "full"}
	for n := 0; n < 10000; n++ {
		f, err := decode(b)
		if err != nil {
			t.Fatal(err)
		}
		a.frame = f
		_ = a.view()
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(a)
	if after.HeapAlloc > before.HeapAlloc+1024*1024 {
		t.Fatalf("retained heap grew by %d", after.HeapAlloc-before.HeapAlloc)
	}
	t.Logf("24-driver display after 10,000 refreshes: retained heap delta %d bytes", int64(after.HeapAlloc)-int64(before.HeapAlloc))
}
