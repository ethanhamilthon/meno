package pathsearch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"main.go", "internal/app/main_test.go", ".git/config", ".env", "node_modules/x/index.js", "docs/my notes.md", "дом/файл.txt", "build/out.bin"} {
		p := filepath.Join(root, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	return root
}

func rels(es []Entry) string {
	var out []string
	for _, e := range es {
		out = append(out, e.Rel)
	}
	return strings.Join(out, ",")
}

func TestSearchIncludesEverythingAndRanksBasenameFirst(t *testing.T) {
	root := tree(t)
	got := rels(Search(context.Background(), root, "main", 10))
	if !strings.HasPrefix(got, "main.go,") && !strings.HasPrefix(got, "internal/app/main_test.go") {
		t.Fatalf("basename matches first: %s", got)
	}
	for _, want := range []string{".git/config", ".env", "node_modules/x/index.js"} {
		if !strings.Contains(rels(Search(context.Background(), root, strings.TrimPrefix(filepath.Base(want), "."), 50)), filepath.ToSlash(want)) {
			t.Errorf("%s must be a candidate: nothing is excluded", want)
		}
	}
	if got := rels(Search(context.Background(), root, "notes", 5)); got != "docs/my notes.md" {
		t.Fatalf("spaces in names: %s", got)
	}
	if got := rels(Search(context.Background(), root, "файл", 5)); got != "дом/файл.txt" {
		t.Fatalf("unicode: %s", got)
	}
	// Deterministic.
	a, b := rels(Search(context.Background(), root, "o", 20)), rels(Search(context.Background(), root, "o", 20))
	if a != b {
		t.Fatalf("order must be deterministic:\n%s\n%s", a, b)
	}
	if exact := Search(context.Background(), root, "main.go", 3); exact[0].Rel != "main.go" {
		t.Fatalf("exact name wins: %+v", exact)
	}
}

func TestSymlinksAreShownNotEnteredAndKeepTheirPath(t *testing.T) {
	root := tree(t)
	os.Symlink(filepath.Join(root, "internal"), filepath.Join(root, "link"))
	os.Symlink(root, filepath.Join(root, "internal", "loop")) // a cycle
	es := Search(context.Background(), root, "link", 10)
	if len(es) == 0 || !es[0].Symlink || !es[0].Dir || es[0].Path != filepath.Join(root, "link") {
		t.Fatalf("the symlink itself is listed with its own path: %+v", es)
	}
	for _, e := range Search(context.Background(), root, "main_test", 20) {
		if strings.HasPrefix(e.Rel, "link/") || strings.Contains(e.Rel, "loop/") {
			t.Fatalf("a symlinked directory must not be searched into: %s", e.Rel)
		}
	}
}

func TestResolveNavigationAndCancellation(t *testing.T) {
	cwd := "/work/proj"
	for q, want := range map[string][3]string{
		"abc":       {"/work/proj", "abc", ""},
		"../x":      {"/work", "x", "../"},
		"~/dev/y":   {"/home/u/dev", "y", "~/dev/"},
		"/etc/pa":   {"/etc", "pa", "/etc/"},
		"sub/dir/f": {"/work/proj/sub/dir", "f", "sub/dir/"},
	} {
		b, f, d := Resolve(cwd, "/home/u", q)
		if b != want[0] || f != want[1] || d != want[2] {
			t.Errorf("%q => %q %q %q, want %v", q, b, f, d, want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := Search(ctx, tree(t), "a", 5); got != nil {
		t.Fatal("a cancelled search returns nothing")
	}
	// An empty fuzzy part lists the children only.
	es := Search(context.Background(), tree(t), "", 50)
	for _, e := range es {
		if strings.Contains(e.Rel, "/") {
			t.Fatalf("children only: %s", e.Rel)
		}
	}
}
