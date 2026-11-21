//go:build desktop && linux

package main

// X11 placement is still requested through the window manager after mapping.
// The Windows-only creation hook has no equivalent in Fyne's public X11 API.
func geometryBeforeShow(title string, p windowPlacement) func() { return func() {} }
