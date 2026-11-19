package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func badgeFixture() []byte {
	b := make([]byte, badgeFrameSize)
	copy(b, "USTBADG2")
	binary.LittleEndian.PutUint32(b[8:], 2)
	binary.LittleEndian.PutUint32(b[12:], badgeFrameSize)
	binary.LittleEndian.PutUint64(b[24:], 42)
	at := 32
	binary.LittleEndian.PutUint64(b[at:], 1234)
	binary.LittleEndian.PutUint64(b[at+8:], 7)
	binary.LittleEndian.PutUint32(b[at+16:], 2)
	binary.LittleEndian.PutUint32(b[at+20:], 1)
	copy(b[at+24:], "eaid://ui_package/art/textures/render_badge_eUserDefined 7")
	copy(b[at+280:], []byte{255, 0, 0, 64, 0, 255, 0, 255})
	return b
}
func badgeFrame() Frame {
	return Frame{Session: 42, State: active, Cars: []Car{{Vehicle: 0, Identity: 1234, Known: knownName | knownPosition | knownTyre, Name: "Test Driver", Position: 1, Compound: 18}}}
}
func testBadgePNG(t *testing.T) string {
	t.Helper()
	f := badgeFrame()
	var cache badgeCache
	cache.apply(badgeFixture(), &f)
	if f.Cars[0].BadgePNG == "" {
		t.Fatal("badge not encoded")
	}
	return f.Cars[0].BadgePNG
}
func checkBadgePNG(t *testing.T, encoded string) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	picture, err := png.Decode(bytes.NewReader(raw))
	if err != nil || picture.Bounds().Dx() != 2 || picture.Bounds().Dy() != 1 {
		t.Fatalf("bad badge: %v", err)
	}
	got := color.NRGBAModel.Convert(picture.At(0, 0)).(color.NRGBA)
	if got != (color.NRGBA{R: 255, A: 64}) {
		t.Fatalf("alpha or colour changed: %+v", got)
	}
}
func TestBadgePixelsAndCache(t *testing.T) {
	b, f := badgeFixture(), badgeFrame()
	var cache badgeCache
	cache.apply(b, &f)
	checkBadgePNG(t, f.Cars[0].BadgePNG)
	first := f.Cars[0].BadgePNG
	// An unchanged revision reuses the exact encoded string; pixels are not encoded each tick.
	b[32+280] = 100
	cache.apply(b, &f)
	if f.Cars[0].BadgePNG != first || len(cache.images) != 1 {
		t.Fatal("unchanged badge was re-encoded")
	}
	binary.LittleEndian.PutUint64(b[40:], 8)
	cache.apply(b, &f)
	if f.Cars[0].BadgePNG == first || len(cache.images) != 1 {
		t.Fatal("changed revision did not replace cached badge")
	}
	binary.LittleEndian.PutUint64(b[24:], 43)
	f.Session = 43
	cache.apply(b, &f)
	if cache.session != 43 || len(cache.images) != 1 {
		t.Fatal("session cache was not reset")
	}
	binary.LittleEndian.PutUint32(b[48:], 0)
	cache.apply(b, &f)
	if f.Cars[0].BadgePNG != "" || len(cache.images) != 0 {
		t.Fatal("unavailable badge was retained")
	}
}
func TestBadgeValidationAndIdentity(t *testing.T) {
	for _, change := range []func([]byte){
		func(b []byte) { b[0] = 'X' }, func(b []byte) { b[8] = 1 }, func(b []byte) { b[12] = 0 },
		func(b []byte) { b[24] = 43 }, func(b []byte) { b[32] = 1 },
		func(b []byte) { binary.LittleEndian.PutUint32(b[48:], 65) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[52:], 65) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[48:], 0) },
		func(b []byte) { b[56] = 0 },
		func(b []byte) {
			for i := 56; i < 312; i++ {
				b[i] = 'x'
			}
		},
	} {
		b, f := badgeFixture(), badgeFrame()
		change(b)
		var cache badgeCache
		cache.apply(b, &f)
		if f.Cars[0].BadgePNG != "" {
			t.Fatal("malformed or wrong-car badge shown")
		}
	}
	f := badgeFrame()
	f.State = 0
	var cache badgeCache
	cache.apply(badgeFixture(), &f)
	if f.Cars[0].BadgePNG != "" {
		t.Fatal("ended session retained a badge")
	}
}
func TestBadgeColumnAndRefresh(t *testing.T) {
	columns := columns(layoutPractice)
	if columns[0]["key"] != "position" || columns[1]["key"] != "team" || columns[2]["key"] != "driver" {
		t.Fatal("team column is misplaced")
	}
	a := App{frame: grid(), mode: "full", status: "Live"}
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.Cars[0].BadgePNG = testBadgePNG(t)
	if !a.needsView() {
		t.Fatal("badge arrival did not refresh the page")
	}
	view := a.view()
	rows := view["body"].(M)["blocks"].([]M)[0]["table"].(M)["rows"].([]M)
	imageData := rows[0]["cells"].([]any)[1].(M)["image_data"].(M)
	if imageData["transparent"] != true || imageData["fit"] != "contain" {
		t.Fatal("badge transparency/aspect settings missing")
	}
	a.lastView = a.viewState()
	a.frame.Cars[0].BadgePNG = ""
	if !a.needsView() || badgeCell(a.frame.Cars[0]) != "" {
		t.Fatal("missing badge was not cleared")
	}
}
func TestBadgeCacheBounded(t *testing.T) {
	var cache badgeCache
	for revision := uint64(0); revision < 200; revision++ {
		b, f := badgeFixture(), badgeFrame()
		binary.LittleEndian.PutUint64(b[40:], revision)
		cache.apply(b, &f)
		if len(cache.images) > 1 {
			t.Fatal("old image revisions accumulated")
		}
	}
}
func TestNativeBadgesABI(t *testing.T) {
	path := os.Getenv("TELEMETRY_BADGES_FRAME")
	if path == "" {
		t.Skip("native image fixture supplied by reader checks")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f := badgeFrame()
	var cache badgeCache
	cache.apply(b, &f)
	checkBadgePNG(t, f.Cars[0].BadgePNG)
	checkBadgePNG(t, f.Cars[0].TyrePNG)
	data, err := json.Marshal(badgeCell(f.Cars[0]))
	if err != nil || !bytes.Contains(data, []byte("image_data")) {
		t.Fatal("native badge did not reach table cell", err)
	}
}

func TestTyreImagesIndependentAndBounded(t *testing.T) {
	b, f := badgeFixture(), badgeFrame()
	at := 32 + maxCars*badgeRecordSize
	copy(b[at:at+badgeRecordSize], b[32:32+badgeRecordSize])
	copy(b[at+24:at+280], make([]byte, 256))
	copy(b[at+24:], "#overrides.tyres.icon.custom_small")
	var cache badgeCache
	cache.apply(b, &f)
	checkBadgePNG(t, f.Cars[0].TyrePNG)
	if len(cache.images) != 2 {
		t.Fatal("shared image cache lost a channel")
	}
	binary.LittleEndian.PutUint64(b[at+8:], 8)
	b[at+280] = 200
	before := f.Cars[0].TyrePNG
	cache.apply(b, &f)
	if f.Cars[0].TyrePNG == before || len(cache.images) != 2 {
		t.Fatal("tyre revision not replaced")
	}
	binary.LittleEndian.PutUint64(b[at:], 999)
	cache.apply(b, &f)
	if f.Cars[0].TyrePNG != "" || f.Cars[0].BadgePNG == "" {
		t.Fatal("wrong-car tyre or missing badge")
	}
	a := App{frame: badgeFrame(), mode: "full"}
	a.frame.Cars[0].TyrePNG = testBadgePNG(t)
	a.lastView, a.viewSent = a.viewState(), true
	a.frame.Cars[0].TyrePNG = ""
	if !a.needsView() {
		t.Fatal("tyre disappearance not refreshed")
	}
	a.hidden[layoutPractice] = fieldBit(layoutPractice, "tyre")
	a.lastView = a.viewState()
	a.frame.Cars[0].TyrePNG = testBadgePNG(t)
	if a.needsView() {
		t.Fatal("hidden tyre image refreshed view")
	}
}
