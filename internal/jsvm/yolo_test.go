package jsvm

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The agent runs without a sandbox: any file the process can reach is fair game.
func TestFSReachesOutsideRoot(t *testing.T) {
	outside := t.TempDir()
	root := filepath.Join(t.TempDir(), "proj")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	f := FS{Root: root}
	abs := filepath.Join(outside, "sub", "a.txt")
	if err := f.Write(abs, "hi"); err != nil {
		t.Fatal(err)
	}
	if got, err := f.Read(abs); err != nil || got != "hi" {
		t.Fatalf("absolute read: %q %v", got, err)
	}
	rel, _ := filepath.Rel(root, abs)
	if got, err := f.Read(rel); err != nil || got != "hi" {
		t.Fatalf("parent-relative read %q: %q %v", rel, got, err)
	}
	if n, err := f.Edit(abs, "hi", "yo"); err != nil || n != 1 {
		t.Fatalf("edit: %d %v", n, err)
	}
	got, err := f.Glob(filepath.Join(outside, "**", "*.txt"))
	if want := []string{abs}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("absolute glob: %v %v want %v", got, err, want)
	}
	t.Setenv("HOME", outside)
	if got, err := f.Read("~/sub/a.txt"); err != nil || got != "yo" {
		t.Fatalf("home read: %q %v", got, err)
	}
}
