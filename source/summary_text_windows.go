//go:build windows

package main

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

// Use the Windows UI font to keep the compact strip legible at high DPI without
// bundling a font or adding a runtime dependency. This draws only our own bitmap.
func drawSummaryText(im *image.RGBA, texts []summaryText) error {
	gdi := syscall.NewLazyDLL("gdi32.dll")
	user := syscall.NewLazyDLL("user32.dll")
	createDC, selectObject, deleteObject := gdi.NewProc("CreateCompatibleDC"), gdi.NewProc("SelectObject"), gdi.NewProc("DeleteObject")
	dc, _, err := createDC.Call(0)
	if dc == 0 {
		return fmt.Errorf("summary drawing context: %v", err)
	}
	defer gdi.NewProc("DeleteDC").Call(dc)
	type bitmapInfo struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		ClrUsed, ClrImportant  uint32
		Colours                uint32
	}
	info := bitmapInfo{Size: 40, Width: int32(im.Rect.Dx()), Height: -int32(im.Rect.Dy()), Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	bitmap, _, err := gdi.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 {
		return fmt.Errorf("summary bitmap: %v", err)
	}
	defer deleteObject.Call(bitmap)
	oldBitmap, _, _ := selectObject.Call(dc, bitmap)
	defer selectObject.Call(dc, oldBitmap)
	pixels := unsafe.Slice((*byte)(bits), len(im.Pix))
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = im.Pix[i+2], im.Pix[i+1], im.Pix[i], 255
	}
	gdi.NewProc("SetBkMode").Call(dc, 1)
	face, _ := syscall.UTF16PtrFromString("Segoe UI")
	type rect struct{ Left, Top, Right, Bottom int32 }
	for _, t := range texts {
		height := -int32(t.Size)
		font, _, e := gdi.NewProc("CreateFontW").Call(uintptr(height), 0, 0, 0, uintptr(t.Weight), 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(face)))
		if font == 0 {
			return fmt.Errorf("summary font: %v", e)
		}
		oldFont, _, _ := selectObject.Call(dc, font)
		gdi.NewProc("SetTextColor").Call(dc, uintptr(t.Colour.R)|uintptr(t.Colour.G)<<8|uintptr(t.Colour.B)<<16)
		text, _ := syscall.UTF16PtrFromString(t.Text)
		box := rect{int32(t.X), int32(t.Y), int32(t.X + t.W), int32(t.Y + t.H)}
		user.NewProc("DrawTextW").Call(dc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&box)), 0x20|0x800|0x8000)
		selectObject.Call(dc, oldFont)
		deleteObject.Call(font)
	}
	for i := 0; i < len(pixels); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = pixels[i+2], pixels[i+1], pixels[i], 255
	}
	return nil
}
