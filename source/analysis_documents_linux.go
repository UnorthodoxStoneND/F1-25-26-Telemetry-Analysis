package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func analysisDocuments() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	file, err := os.Open(filepath.Join(config, "user-dirs.dirs"))
	if os.IsNotExist(err) {
		return filepath.Join(home, "Documents"), nil
	}
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if value, ok := strings.CutPrefix(strings.TrimSpace(scanner.Text()), "XDG_DOCUMENTS_DIR="); ok {
			if path := xdgDocumentsPath(home, value); path != "" {
				return path, nil
			}
		}
	}
	return filepath.Join(home, "Documents"), scanner.Err()
}

func xdgDocumentsPath(home, value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return ""
	}
	value = value[1 : len(value)-1]
	// Read the XDG assignment as data. Expand only the documented HOME prefix.
	if value == "$HOME" {
		return home
	}
	if tail, ok := strings.CutPrefix(value, "$HOME/"); ok {
		return filepath.Join(home, tail)
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return ""
}
