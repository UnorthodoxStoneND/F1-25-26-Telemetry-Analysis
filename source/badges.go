package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/png"
)

const badgeFrameSize = 533280
const badgeRecordSize = 16664

type badgeKey struct {
	Reference     string
	Revision      uint64
	Width, Height uint32
}
type badgeCache struct {
	session uint64
	images  map[badgeKey]string
}

func (cache *badgeCache) apply(b []byte, f *Frame) {
	for n := range f.Cars {
		f.Cars[n].BadgePNG = ""
	}
	if f.State&active == 0 {
		*cache = badgeCache{}
		return
	}
	if len(b) != badgeFrameSize || string(b[:8]) != "USTBADG1" ||
		binary.LittleEndian.Uint32(b[8:]) != 1 || binary.LittleEndian.Uint32(b[12:]) != badgeFrameSize ||
		binary.LittleEndian.Uint64(b[24:]) != f.Session {
		return
	}
	if cache.session != f.Session {
		cache.images = nil
		cache.session = f.Session
	}
	if cache.images == nil {
		cache.images = make(map[badgeKey]string)
	}
	used := make(map[badgeKey]bool)
	for n := range f.Cars {
		c := &f.Cars[n]
		if c.Vehicle >= maxCars {
			continue
		}
		at := 32 + int(c.Vehicle)*badgeRecordSize
		if c.Identity == 0 || binary.LittleEndian.Uint64(b[at:]) != c.Identity {
			continue
		}
		width := binary.LittleEndian.Uint32(b[at+16:])
		height := binary.LittleEndian.Uint32(b[at+20:])
		reference := b[at+24 : at+280]
		if width == 0 || height == 0 || width > 64 || height > 64 || reference[0] == 0 || bytes.IndexByte(reference, 0) < 0 {
			continue
		}
		key := badgeKey{cstring(reference), binary.LittleEndian.Uint64(b[at+8:]), width, height}
		encoded, ok := cache.images[key]
		if !ok {
			// Live State gives straight RGBA8. NRGBA preserves colour and transparency.
			pixels := b[at+280 : at+280+int(width*height*4)]
			picture := image.NRGBA{Pix: pixels, Stride: int(width) * 4, Rect: image.Rect(0, 0, int(width), int(height))}
			var out bytes.Buffer
			encoder := png.Encoder{CompressionLevel: png.BestSpeed}
			if encoder.Encode(&out, &picture) != nil {
				continue
			}
			encoded = base64.StdEncoding.EncodeToString(out.Bytes())
			if len(encoded) > 64*1024 {
				continue
			}
			cache.images[key] = encoded
		}
		used[key] = true
		c.BadgePNG = encoded
	}
	for key := range cache.images {
		if !used[key] {
			delete(cache.images, key)
		}
	}
}

func badgeRowReady(c Car) bool {
	const basics = knownName | knownPosition | knownTyre
	return c.Known&basics == basics && c.Name != "" && c.Position > 0 &&
		(c.Compound == 7 || c.Compound == 8 || (c.Compound >= 16 && c.Compound <= 20))
}

func badgeCell(c Car) any {
	if c.BadgePNG == "" || !badgeRowReady(c) {
		return ""
	}
	return M{"image_data": M{"png": c.BadgePNG, "width": 20, "height": 20, "fit": "contain", "transparent": true}}
}
