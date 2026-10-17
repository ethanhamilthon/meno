package jsvm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSetRootMovesFsAndShKeepsVariables(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(a, "f.txt"), []byte("in a"), 0o644)
	os.WriteFile(filepath.Join(b, "f.txt"), []byte("in b"), 0o644)
	v := New(Options{Root: a})
	if r := v.Exec(context.Background(), "let keep = 41; fs.read('f.txt')"); r.Value != "in a" || r.Err != "" {
		t.Fatalf("%+v", r)
	}
	v.SetRoot(b)
	if v.Root() != b {
		t.Fatal("root")
	}
	if r := v.Exec(context.Background(), "fs.read('f.txt') + '/' + sh('pwd').stdout.trim() + '/' + (keep + 1)"); r.Err != "" || r.Value != "in b/"+realPath(t, b)+"/42" {
		t.Fatalf("%+v", r)
	}
}

func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
