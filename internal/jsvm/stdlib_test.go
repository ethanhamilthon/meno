package jsvm

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTopLevelRedeclarationAllowedInBatchedCall(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	r := ex(t, v, "const r = 10; const r = 20; let x = 1; let x = 2; r + x")
	if r.Err != "" || r.Value != "22" {
		t.Fatalf("top-level redeclaration: %+v", r)
	}
	r = ex(t, v, "const a = 3; const a = 4; return a * 2")
	if r.Err != "" || r.Value != "8" {
		t.Fatalf("wrapped redeclaration: %+v", r)
	}
}

func TestHelpAliasesAndHints(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	for _, alias := range []string{"sh", "view", "websearch", "hash", "path", "base64", "btoa"} {
		r := ex(t, v, "help('"+alias+"')")
		if r.Err != "" || !strings.HasPrefix(r.Value, "# js") {
			t.Fatalf("alias %s: %+v", alias, r)
		}
	}
	if r := ex(t, v, "help('ask')"); r.Err != "" || !strings.HasPrefix(r.Value, "# askUser") {
		t.Fatalf("ask alias: %+v", r)
	}
	if r := ex(t, v, "help('fs.patch')"); r.Err != "" || !strings.HasPrefix(r.Value, "# fs") {
		t.Fatalf("dotted alias: %+v", r)
	}
	if r := ex(t, v, "help('unknown_thing')"); !strings.Contains(r.Err, "help(\"js\")") {
		t.Fatalf("hint missing: %+v", r)
	}
}

func TestFSWriteRefusesOverwriteWithoutFlag(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	if r := ex(t, v, "fs.write('a.txt', 'first')"); r.Err != "" {
		t.Fatal(r.Err)
	}
	if r := ex(t, v, "fs.write('a.txt', 'second')"); !strings.Contains(r.Err, "already exists") || !strings.Contains(r.Err, "overwrite: true") {
		t.Fatalf("expected overwrite error, got %+v", r)
	}
	if r := ex(t, v, "fs.read('a.txt')"); r.Value != "first" {
		t.Fatalf("file was overwritten: %q", r.Value)
	}
	if r := ex(t, v, "fs.write('a.txt', 'second', {overwrite: true}); fs.read('a.txt')"); r.Err != "" || r.Value != "second" {
		t.Fatalf("overwrite option failed: %+v", r)
	}
	if r := ex(t, v, "fs.write('a.txt', 'third', true); fs.read('a.txt')"); r.Err != "" || r.Value != "third" {
		t.Fatalf("overwrite bool failed: %+v", r)
	}
}

func TestStdlibHashPathBase64UTF8(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "sub/hello.txt", "привет мир")
	v := New(Options{Root: dir})

	r := ex(t, v, "hash('abc') === hash.sha256('abc') && hash('abc').length === 64 && hash.md5('abc').length === 32 && hash.sha1('abc').length === 40")
	if r.Err != "" || r.Value != "true" {
		t.Fatalf("hash: %+v", r)
	}
	if r := ex(t, v, "hash.file('sub/hello.txt') === fs.inspect('sub/hello.txt').sha256"); r.Err != "" || r.Value != "true" {
		t.Fatalf("hash.file: %+v", r)
	}
	if r := ex(t, v, "path.join('a', 'b', 'c.go') + '|' + path.dirname('a/b/c.go') + '|' + path.basename('a/b/c.go', '.go') + '|' + path.extname('a/b/c.go')"); r.Err != "" || r.Value != "a/b/c.go|a/b|c|.go" {
		t.Fatalf("path: %+v", r)
	}
	if r := ex(t, v, "path.relative(path.resolve('sub/hello.txt'))"); r.Err != "" || r.Value != "sub/hello.txt" {
		t.Fatalf("path.resolve/relative: %+v", r)
	}
	if r := ex(t, v, "const s = 'Привет, 🦊 UTF-8!'; base64.decode(base64.encode(s)) === s && atob(btoa(s)) === s && base64.decodeURL(base64.encodeURL(s)) === s"); r.Err != "" || r.Value != "true" {
		t.Fatalf("base64 utf8: %+v", r)
	}
}

func TestTimeoutErrorMentionsTasks(t *testing.T) {
	v := New(Options{Root: t.TempDir(), Timeout: 50 * time.Millisecond})
	r := v.Exec(context.Background(), "while(true){}")
	if !r.TimedOut || !strings.Contains(r.Err, "tasks.start") {
		t.Fatalf("timeout hint missing: %+v", r)
	}
}

func TestPatchShowsWhereOldTextStoppedMatching(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "p.txt", "line one\nline two\nline three actual\nline four\n")
	v := New(Options{Root: dir})
	h := ex(t, v, "fs.inspect('p.txt').sha256").Value
	r := ex(t, v, "fs.patch('p.txt', [{oldText: 'line one\\nline two\\nline three wrong\\nline four', newText: 'x'}], {expectedHash: '"+h+"'})")
	if !strings.Contains(r.Err, "line 1") || !strings.Contains(r.Err, "stopped matching at line 3") || !strings.Contains(r.Err, "line three actual") || !strings.Contains(r.Err, "line three wrong") {
		t.Fatalf("patch diff diagnostic: %+v", r)
	}
}
