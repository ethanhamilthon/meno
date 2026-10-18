package jsvm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadAndInspectRanges(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.txt", "один\nдва\nthree\n")
	v := New(Options{Root: dir})
	for code, want := range map[string]string{
		"fs.read('a.txt')":                              "один\nдва\nthree\n",
		"fs.read('a.txt', {startLine: 2})":              "два\nthree\n",
		"fs.read('a.txt', {startLine: 2, endLine: 2})":  "два\n",
		"fs.read('a.txt', {endLine: 1})":                "один\n",
		"fs.read('a.txt', {startLine: 3, endLine: 99})": "three\n",
		"fs.inspect('a.txt', {startLine: 2, endLine: 3}).startLine + '-' + fs.inspect('a.txt', {startLine: 2, endLine: 3}).endLine": "2-3",
	} {
		if r := ex(t, v, code); r.Err != "" || r.Value != want {
			t.Errorf("%s => %+v", code, r)
		}
	}
	for _, bad := range []string{"{startLine: 0, endLine: 0, x: 1}", "{startLine: 3, endLine: 2}", "{startLine: 9}", "{startLine: -1}"} {
		if r := ex(t, v, "fs.read('a.txt', "+bad+")"); r.Err == "" {
			t.Errorf("%s must fail", bad)
		}
	}
	r := ex(t, v, "fs.inspect('a.txt', {startLine: 1, endLine: 1}).sha256")
	whole := ex(t, v, "fs.inspect('a.txt').sha256")
	if r.Value != whole.Value || len(r.Value) != 64 {
		t.Fatalf("the hash covers the whole file: %q vs %q", r.Value, whole.Value)
	}
}

func TestPatchStaleHashAmbiguityOverlapAndAtomicity(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "f.go", "alpha\nbeta\nbeta\ngamma\n")
	os.Chmod(p, 0o755)
	v := New(Options{Root: dir})
	h := ex(t, v, "fs.inspect('f.go').sha256").Value
	if r := ex(t, v, "fs.patch('f.go', [{oldText: 'beta', newText: 'x'}], {expectedHash: '"+h+"'})"); !strings.Contains(r.Err, "2 times") || !strings.Contains(r.Err, "line 2") {
		t.Fatalf("ambiguous: %+v", r)
	}
	if r := ex(t, v, "fs.patch('f.go', [{oldText: 'alpha', newText: 'A'}, {oldText: 'zzz', newText: 'B'}], {expectedHash: '"+h+"'})"); r.Err == "" {
		t.Fatal("a missing text must fail")
	}
	if b, _ := os.ReadFile(p); string(b) != "alpha\nbeta\nbeta\ngamma\n" {
		t.Fatalf("a failed patch wrote: %q", b)
	}
	if r := ex(t, v, "fs.patch('f.go', [{oldText: 'alpha\\nbeta\\nbeta', newText: 'A'}, {oldText: 'gamma', newText: 'G'}, {oldText: 'beta\\nbeta\\ngamma', newText: 'X'}], {expectedHash: '"+h+"'})"); !strings.Contains(r.Err, "overlap") {
		t.Fatalf("overlap: %+v", r)
	}
	if r := ex(t, v, "fs.patch('f.go', [{oldText: '', newText: 'x'}], {expectedHash: '"+h+"'})"); !strings.Contains(r.Err, "empty") {
		t.Fatalf("empty: %+v", r)
	}
	if r := ex(t, v, "fs.patch('f.go', [{oldText: 'alpha', newText: 'A'}])"); !strings.Contains(r.Err, "expectedHash is required") {
		t.Fatalf("hash required: %+v", r)
	}
	r := ex(t, v, "const x = fs.patch('f.go', [{oldText: 'alpha', newText: 'A'}, {oldText: 'gamma', newText: 'G'}], {expectedHash: '"+h+"'}); x.replacements + ':' + x.sha256")
	if r.Err != "" || !strings.HasPrefix(r.Value, "2:") {
		t.Fatalf("%+v", r)
	}
	if b, _ := os.ReadFile(p); string(b) != "A\nbeta\nbeta\nG\n" {
		t.Fatalf("%q", b)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o755 {
		t.Fatalf("permissions lost: %v", st.Mode())
	}
	if r := ex(t, v, "fs.patch('f.go', [{oldText: 'A', newText: 'B'}], {expectedHash: '"+h+"'})"); !strings.Contains(r.Err, "changed since") {
		t.Fatalf("stale hash: %+v", r)
	}
}

func TestEditDiagnostics(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "e.txt", "one\ntwo words\ntwo words\n")
	v := New(Options{Root: dir})
	if r := ex(t, v, "fs.edit('e.txt', 'two words', 'x')"); !strings.Contains(r.Err, "2 times") {
		t.Fatalf("%+v", r)
	}
	if r := ex(t, v, "fs.edit('e.txt', 'one\\nthree', 'x')"); !strings.Contains(r.Err, "line 1") {
		t.Fatalf("nearest line: %+v", r)
	}
	if r := ex(t, v, "fs.edit('e.txt', 'ONE', 'x')"); r.Err == "" {
		t.Fatal("no silent replace-all or fuzzy edit")
	}
}

func TestSearchOrderPaginationUnicodeAndIgnored(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "b.txt", "héllo needle\nneedle needle\n")
	write(t, dir, "a.txt", "no\nNeedle here\n")
	write(t, dir, "bin.dat", "needle\x00binary")
	write(t, dir, "node_modules/x.txt", "needle")
	v := New(Options{Root: dir})
	r := ex(t, v, "JSON.stringify(fs.search('needle').matches.map(m => m.path + ':' + m.line + ':' + m.column))")
	if r.Value != "[\"b.txt:1:7\",\"b.txt:2:1\",\"b.txt:2:8\"]" {
		t.Fatalf("%q", r.Value)
	}
	r = ex(t, v, "fs.search('needle', {caseSensitive: false, includeIgnored: true}).matches.length")
	if r.Value != "5" {
		t.Fatalf("case-insensitive with ignored: %q", r.Value)
	}
	r = ex(t, v, "let cur; const sizes = []; for (let i = 0; i < 10; i++) { const p = fs.search('needle', {limit: 2, cursor: cur}); sizes.push(p.matches.length + String(p.truncated)); cur = p.nextCursor; if (!cur) break; } sizes.join(',')")
	if r.Err != "" || r.Value != "2true,1false" {
		t.Fatalf("pagination: %+v", r)
	}
	if r := ex(t, v, "fs.search('(', {regex: true})"); !strings.Contains(r.Err, "bad regex") {
		t.Fatalf("%+v", r)
	}
	if r := ex(t, v, "fs.search('a', {bogus: 1})"); !strings.Contains(r.Err, "unknown option") {
		t.Fatalf("%+v", r)
	}
}
