package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestJSONRoundTripAndMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "data.json")
	got := map[string]string{"default": "kept"}
	if err := LoadJSON(path, &got); err != nil || got["default"] != "kept" {
		t.Fatalf("missing: %v %v", got, err)
	}
	if err := SaveJSON(path, map[string]string{"secret": "value"}); err != nil {
		t.Fatal(err)
	}
	got = nil
	if err := LoadJSON(path, &got); err != nil || got["secret"] != "value" {
		t.Fatalf("round trip: %v %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions: %v", info.Mode())
	}
}

func TestSaveJSONFailuresPreserveDestinationAndCleanTemps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := SaveJSON(path, "original"); err != nil {
		t.Fatal(err)
	}
	if err := SaveJSON(path, make(chan int)); err == nil {
		t.Fatal("marshal unexpectedly succeeded")
	}
	var got string
	if err := LoadJSON(path, &got); err != nil || got != "original" {
		t.Fatalf("destination changed: %q %v", got, err)
	}
	blocked := filepath.Join(dir, "directory")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := SaveJSON(blocked, "new"); err == nil {
		t.Fatal("rename unexpectedly succeeded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary files leaked: %v %v", entries, err)
	}
}

func TestSaveJSONConcurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := SaveJSON(path, "initial"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := SaveJSON(path, fmt.Sprintf("writer-%d", i)); err != nil {
				t.Error(err)
			}
			b, err := os.ReadFile(path)
			if err != nil || !json.Valid(b) {
				t.Errorf("partial JSON: %q %v", b, err)
			}
		}(i)
	}
	wg.Wait()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "data.json" {
		t.Fatalf("temporary files leaked: %v %v", entries, err)
	}
}
