package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalysisDocumentsUsesXDG(t *testing.T) {
	home, config := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	got, err := analysisDocuments()
	if err != nil || got != filepath.Join(home, "Documents") {
		t.Fatalf("fallback: %q %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(config, "user-dirs.dirs"), []byte("XDG_DOCUMENTS_DIR=\"$HOME/My Documents\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = analysisDocuments()
	if err != nil || got != filepath.Join(home, "My Documents") {
		t.Fatalf("XDG Documents: %q %v", got, err)
	}
}
