package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentsStorageMigration(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "Redirected Documents")
	t.Setenv("F1_TELEMETRY_TEST_DOCUMENTS", docs)
	old := filepath.Join(root, "Swofty", "addon-data", "unorthodoxstonendmodding.udp-telemetry")
	put := func(path, value string) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, []byte(value), 0644); e != nil {
			t.Fatal(e)
		}
	}
	put(filepath.Join(old, "settings.json"), `{"port":20800}`)
	put(filepath.Join(old, "recordings", "one.json"), "metadata")
	put(filepath.Join(old, "recordings", "one.udp.gz"), "journal")
	put(filepath.Join(old, "trace-one.png"), "chart")
	dest := filepath.Join(docs, "F1 Game Telemetry")
	put(filepath.Join(dest, "settings.json"), `{"port":20900}`)
	got, e := prepareStorage(old)
	if e != nil || got != dest {
		t.Fatalf("destination %q: %v", got, e)
	}
	for path, want := range map[string]string{"settings.json": `{"port":20900}`, "recordings/one.json": "metadata", "recordings/one.udp.gz": "journal", "trace-one.png": "chart"} {
		b, e := os.ReadFile(filepath.Join(dest, filepath.FromSlash(path)))
		if e != nil || string(b) != want {
			t.Fatalf("%s: %q %v", path, b, e)
		}
	}
	if _, e := os.Stat(filepath.Join(old, "recordings", "one.udp.gz")); e != nil {
		t.Fatal("original recording removed", e)
	}
	if e := os.Remove(filepath.Join(dest, "recordings", "one.json")); e != nil {
		t.Fatal(e)
	}
	if _, e := prepareStorage(old); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(dest, "recordings", "one.json")); !os.IsNotExist(e) {
		t.Fatal("deleted recording was imported again", e)
	}
}

func TestStorageWithoutLegacyAndNestedRejection(t *testing.T) {
	root := t.TempDir()
	t.Setenv("F1_TELEMETRY_TEST_DOCUMENTS", filepath.Join(root, "Documents"))
	if _, e := prepareStorage(filepath.Join(root, "missing")); e != nil {
		t.Fatal(e)
	}
	if e := migrateStorage(root, filepath.Join(root, "child")); e == nil {
		t.Fatal("accepted nested storage paths")
	}
}
