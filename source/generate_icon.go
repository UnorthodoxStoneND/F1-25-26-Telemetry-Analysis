//go:build ignore

// Build-time conversion of the launcher artwork for the desktop window and
// the separate custom artwork for the Windows executable icon.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

// Windows PNG icon entries advertise 32 bpp. Keep an alpha channel even when
// the approved artwork has a fully opaque black background.
type exeIconRGBA struct{ *image.NRGBA }

func (exeIconRGBA) Opaque() bool { return false }

func main() {
	if err := generate(); err != nil {
		panic(err)
	}
}
func generate() error {
	input, err := os.Open("../icon.png")
	if err != nil {
		return err
	}
	original, err := png.Decode(input)
	input.Close()
	if err != nil {
		return err
	}
	window := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	draw.CatmullRom.Scale(window, window.Bounds(), original, original.Bounds(), draw.Src, nil)
	var windowPNG bytes.Buffer
	if err := png.Encode(&windowPNG, window); err != nil {
		return err
	}
	if err := os.WriteFile("retro_icon.png", windowPNG.Bytes(), 0644); err != nil {
		return err
	}
	exeInput, err := os.Open("exe_icon.png")
	if err != nil {
		return err
	}
	exeArtwork, err := png.Decode(exeInput)
	exeInput.Close()
	if err != nil {
		return err
	}
	sizes := []int{16, 32, 48, 256}
	images := make([][]byte, len(sizes))
	for i, size := range sizes {
		scaled := image.NewNRGBA(image.Rect(0, 0, size, size))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), exeArtwork, exeArtwork.Bounds(), draw.Src, nil)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, exeIconRGBA{scaled}); err != nil {
			return err
		}
		images[i] = encoded.Bytes()
		// Conventional Windows ICO layout: uncompressed 32-bit DIBs for
		// small shell icons and a 32-bit PNG for the 256px image.
		if size < 256 {
			images[i] = iconDIB(scaled)
		}
	}
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, uint16(0))
	binary.Write(&ico, binary.LittleEndian, uint16(1))
	binary.Write(&ico, binary.LittleEndian, uint16(len(sizes)))
	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		ico.Write([]byte{byte(size % 256), byte(size % 256), 0, 0})
		binary.Write(&ico, binary.LittleEndian, uint16(1))
		binary.Write(&ico, binary.LittleEndian, uint16(32))
		binary.Write(&ico, binary.LittleEndian, uint32(len(images[i])))
		binary.Write(&ico, binary.LittleEndian, uint32(offset))
		offset += len(images[i])
	}
	for _, data := range images {
		ico.Write(data)
	}
	dir, err := filepath.Abs("../.verify")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "helper-icon.ico")
	if err = os.WriteFile(path, ico.Bytes(), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "helper-icon.rc"), []byte(fmt.Sprintf("1 ICON %q\n", filepath.ToSlash(path))), 0644)
}

func iconDIB(im *image.NRGBA) []byte {
	size := im.Bounds().Dx()
	maskStride := (size + 31) / 32 * 4
	pixelSize := size * size * 4
	data := make([]byte, 40+pixelSize+maskStride*size)
	binary.LittleEndian.PutUint32(data[0:], 40)
	binary.LittleEndian.PutUint32(data[4:], uint32(size))
	binary.LittleEndian.PutUint32(data[8:], uint32(size*2))
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint16(data[14:], 32)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := im.NRGBAAt(x, size-1-y)
			at := 40 + (y*size+x)*4
			copy(data[at:at+4], []byte{c.B, c.G, c.R, c.A})
			if c.A == 0 {
				data[40+pixelSize+y*maskStride+x/8] |= 0x80 >> uint(x%8)
			}
		}
	}
	return data
}
