package main

// The Windows Known Folder API honours OneDrive and redirected Documents folders.
import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func documentsDirectory() (string, error) {
	// Isolate compiled-helper integration fixtures from the user's real Documents.
	if p := os.Getenv("F1_TELEMETRY_TEST_DOCUMENTS"); p != "" {
		if !filepath.IsAbs(p) {
			return "", fmt.Errorf("test Documents path must be absolute")
		}
		return p, nil
	}
	id := syscall.GUID{Data1: 0xFDD39AD0, Data2: 0x238F, Data3: 0x46AF, Data4: [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}}
	var value *uint16
	r, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("SHGetKnownFolderPath").Call(uintptr(unsafe.Pointer(&id)), 0, 0, uintptr(unsafe.Pointer(&value)))
	if r != 0 || value == nil {
		return "", fmt.Errorf("cannot locate Windows Documents folder (0x%x)", r)
	}
	defer syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(value)))
	units := unsafe.Slice(value, 32768)
	for i, ch := range units {
		if ch == 0 {
			return syscall.UTF16ToString(units[:i]), nil
		}
	}
	return "", fmt.Errorf("invalid Windows Documents folder path")
}

func prepareStorage(legacy string) (string, error) {
	documents, err := documentsDirectory()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(documents, "F1 Game Telemetry")
	if err = os.MkdirAll(dest, 0755); err != nil {
		return "", fmt.Errorf("create telemetry folder: %w", err)
	}
	if err = migrateStorage(legacy, dest); err != nil {
		return "", fmt.Errorf("migrate telemetry files: %w", err)
	}
	return dest, nil
}

// Copy once, retaining the original as a backup and never overwriting new files.
// A marker prevents deliberately deleted recordings from reappearing on restart.
func migrateStorage(source, dest string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return err
	}
	if strings.EqualFold(source, dest) {
		return nil
	}
	for _, pair := range [][2]string{{source, dest}, {dest, source}} {
		rel, e := filepath.Rel(pair[0], pair[1])
		if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("storage folders must not contain each other")
		}
	}
	hash := sha256.Sum256([]byte(strings.ToLower(source)))
	marker := filepath.Join(dest, fmt.Sprintf(".migration-%x.json", hash[:8]))
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse linked storage entry: %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported storage entry: %s", path)
		}
		return copyStorageFile(path, target)
	})
	if err != nil {
		return err
	}
	return atomicJSON(marker, M{"source": source, "completed": true})
}

func copyStorageFile(source, target string) (err error) {
	if _, err = os.Lstat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(target), ".migration-copy-*")
	if err != nil {
		return err
	}
	temp := out.Name()
	defer os.Remove(temp)
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	if e := out.Close(); err == nil {
		err = e
	}
	if err == nil {
		// On Windows Rename refuses an existing destination. A concurrent helper
		// completing this copy first is harmless; never replace its file.
		err = os.Rename(temp, target)
		if err != nil {
			if _, exists := os.Lstat(target); exists == nil {
				return nil
			}
		}
	}
	return err
}
