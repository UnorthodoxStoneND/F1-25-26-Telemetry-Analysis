package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// Swofty 1.9.42's ImageRoots only accepts paths under the launcher's supplied
// cache/data roots. Documents remains the authoritative storage location; only
// disposable image copies are published to the permitted display cache.
func (a *App) displayImage(source string) (string, error) {
	if a.displayCache == "" {
		return source, nil
	}
	b, e := readTeamFile(source, 32<<20)
	if e != nil {
		return "", e
	}
	hash := sha256.Sum256(b)
	path := filepath.Join(a.displayCache, fmt.Sprintf("%x.png", hash))
	if cached, e := os.ReadFile(path); e == nil && bytes.Equal(cached, b) {
		return path, nil
	}
	if e = os.MkdirAll(a.displayCache, 0755); e != nil {
		return "", e
	}
	file, e := os.CreateTemp(a.displayCache, ".image-*")
	if e != nil {
		return "", e
	}
	temp := file.Name()
	defer os.Remove(temp)
	_, e = file.Write(b)
	if closeErr := file.Close(); e == nil {
		e = closeErr
	}
	if e == nil {
		e = os.Rename(temp, path)
	}
	if e != nil {
		return "", e
	}
	return path, nil
}
