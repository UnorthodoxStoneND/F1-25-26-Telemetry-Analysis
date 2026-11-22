//go:build ignore

// Build-time conversion of the existing add-on artwork for desktop/PE icons.
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
	sizes := []int{16, 32, 48, 256}
	images := make([][]byte, len(sizes))
	for i, size := range sizes {
		scaled := image.NewNRGBA(image.Rect(0, 0, size, size))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), original, original.Bounds(), draw.Src, nil)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, scaled); err != nil {
			return err
		}
		images[i] = encoded.Bytes()
	}
	if err := os.WriteFile("retro_icon.png", images[len(images)-1], 0644); err != nil {
		return err
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
