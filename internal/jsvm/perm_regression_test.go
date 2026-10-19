package jsvm

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func patchFile(t *testing.T, fs FS, path, oldText, newText string) error {
	t.Helper()
	got, err := fs.Inspect(path, 0, 0)
	if err != nil {
		return err
	}
	_, _, err = fs.Patch(path, []Replacement{{Old: oldText, New: newText}}, got.SHA256)
	return err
}

func TestFSInternalSymlinksAndSymlinkRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "file.txt"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"file.txt": "data/file.txt", "directory": "data"} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, workingRoot := range []string{root, alias} {
		t.Run(workingRoot, func(t *testing.T) {
			fs := FS{Root: workingRoot}
			for _, path := range []string{"file.txt", "directory/file.txt", filepath.Join(alias, "file.txt"), filepath.Join(realRoot, "data", "file.txt")} {
				if _, err := fs.Read(path); err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				if !fs.Exists(path) {
					t.Fatalf("exists %s: false", path)
				}
			}
			if err := fs.Write("file.txt", "original", true); err != nil {
				t.Fatal(err)
			}
			if err := patchFile(t, fs, "directory/file.txt", "original", "edited"); err != nil {
				t.Fatalf("patch: %v", err)
			}
			if got, err := fs.Read("data/file.txt"); err != nil || got != "edited" {
				t.Fatalf("read edited: %q, %v", got, err)
			}
			if err := fs.Write("directory/new/deep.txt", "created", true); err != nil {
				t.Fatal(err)
			}
			if got, err := fs.Read("data/new/deep.txt"); err != nil || got != "created" {
				t.Fatalf("read created: %q, %v", got, err)
			}
			got, err := fs.Glob("**/*.txt")
			want := []string{"data/file.txt", "data/new/deep.txt", "file.txt"}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("glob: %v, %v; want %v", got, err, want)
			}
		})
	}
}

func TestFSExternalPathsAndSymlinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	victim := filepath.Join(outside, "file.txt")
	if err := os.WriteFile(victim, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	fs := FS{Root: root}
	for _, path := range []string{victim, "link/file.txt", filepath.Join("..", filepath.Base(outside), "file.txt")} {
		if got, err := fs.Read(path); err != nil || got != "original" {
			t.Fatalf("read %s: %q, %v", path, got, err)
		}
		if !fs.Exists(path) {
			t.Fatalf("exists %s: false", path)
		}
		if err := patchFile(t, fs, path, "original", "changed"); err != nil {
			t.Fatalf("patch %s: %v", path, err)
		}
		if err := fs.Write(path, "original", true); err != nil {
			t.Fatal(err)
		}
	}
	if err := fs.Write("link/new/deep.txt", "created"); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(outside, "new/deep.txt")); err != nil || string(b) != "created" {
		t.Fatalf("external write: %q, %v", b, err)
	}
}

func TestFSInvalidSymlinksReturnErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("loop", filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}
	fs := FS{Root: root}
	if _, err := fs.Read("loop"); err == nil {
		t.Fatal("read symlink loop succeeded")
	}
	if err := fs.Write("loop", "data"); err == nil {
		t.Fatal("write symlink loop succeeded")
	}
	if fs.Exists("loop") {
		t.Fatal("symlink loop exists")
	}
	if err := os.Symlink(filepath.Join(root, "new.txt"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := fs.Write("dangling", "created"); err != nil {
		t.Fatal(err)
	}
	if got, err := fs.Read("new.txt"); err != nil || got != "created" {
		t.Fatalf("dangling target: %q, %v", got, err)
	}
}
