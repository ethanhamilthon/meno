package jsvm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newVM(t *testing.T) *VM {
	t.Helper()
	root := t.TempDir()
	return New(Options{Root: root, Timeout: 2 * time.Second})
}

func TestStatePersistsUntilReset(t *testing.T) {
	v := newVM(t)
	ctx := context.Background()
	v.Exec(ctx, "var n = 2; const c = 3; let l = 4; function f() { return 5 } class K { v() { return 6 } } globalThis.g = 7")
	for probe, want := range map[string]string{"n": "2", "c": "3", "l": "4", "f()": "5", "new K().v()": "6", "g": "7"} {
		if r := v.Exec(ctx, probe); r.Err != "" || r.Value != want {
			t.Fatalf("%s: %+v, want %s", probe, r, want)
		}
	}
	v.Reset()
	for _, probe := range []string{"typeof n", "typeof c", "typeof l", "typeof f", "typeof K", "typeof g"} {
		if r := v.Exec(ctx, probe); r.Value != "undefined" {
			t.Fatalf("%s survived Reset: %+v", probe, r)
		}
	}
}

func TestRedeclaringInLaterCalls(t *testing.T) {
	v := newVM(t)
	ctx := context.Background()
	for i, code := range []string{"const a = 1; let b = 2; class C {}; a + b", "const a = 10; let b = 20; class C { x() { return 1 } }; a + b", "let b; typeof b"} {
		r := v.Exec(ctx, code)
		want := []string{"3", "30", "undefined"}[i]
		if r.Err != "" || r.Value != want {
			t.Fatalf("call %d: %+v, want %s", i, r, want)
		}
	}
	if r := v.Exec(ctx, "const z;"); r.Err == "" {
		t.Fatal("a const without a value must stay a syntax error")
	}
}

func TestWrappedCodeKeepsItsDeclarationsLocal(t *testing.T) {
	v := newVM(t)
	ctx := context.Background()
	if r := v.Exec(ctx, "var inner = 1; globalThis.shared = 2; return inner"); r.Err != "" || r.Value != "1" {
		t.Fatalf("%+v", r)
	}
	if r := v.Exec(ctx, "[typeof inner, typeof shared].join()"); r.Value != "undefined,number" {
		t.Fatalf("%+v", r)
	}
}

func TestOutputIsPerCall(t *testing.T) {
	v := newVM(t)
	ctx := context.Background()
	v.Exec(ctx, "console.log('first')")
	if r := v.Exec(ctx, "console.log('second')"); r.Output != "second\n" {
		t.Fatalf("output %q", r.Output)
	}
}

func TestStateSurvivesATimeout(t *testing.T) {
	v := newVM(t)
	ctx := context.Background()
	v.Exec(ctx, "var kept = 42")
	if r := v.ExecTimeout(ctx, "while (true) {}", 50*time.Millisecond); !r.TimedOut {
		t.Fatalf("%+v", r)
	}
	if r := v.Exec(ctx, "kept"); r.Err != "" || r.Value != "42" {
		t.Fatalf("%+v", r)
	}
}

func TestPrintCaptured(t *testing.T) {
	v := newVM(t)
	r := v.Exec(context.Background(), `console.log("a", 1, true)`)
	if r.Output != "a 1 true\n" {
		t.Fatalf("output %q", r.Output)
	}
}

func TestFSWriteReadPatchGlob(t *testing.T) {
	v := newVM(t)
	ctx := context.Background()
	r := v.Exec(ctx, `fs.write("src/a.ts", "hello rateLimit"); fs.write("src/b.ts", "x")`)
	if r.Err != "" {
		t.Fatal(r.Err)
	}
	h := v.Exec(ctx, `fs.inspect("src/a.ts").sha256`).Value
	r = v.Exec(ctx, `fs.patch("src/a.ts", [{oldText: "hello", newText: "bye"}], {expectedHash: "`+h+`"})`)
	if r.Err != "" {
		t.Fatal(r.Err)
	}
	r = v.Exec(ctx, `fs.read("src/a.ts")`)
	if r.Value != "bye rateLimit" {
		t.Fatalf("read %q", r.Value)
	}
	r = v.Exec(ctx, `fs.glob("src/*.ts").join(",")`)
	if r.Value != "src/a.ts,src/b.ts" {
		t.Fatalf("glob %q", r.Value)
	}
	r = v.Exec(ctx, `fs.exists("src/nope.ts")`)
	if r.Value != "false" {
		t.Fatalf("exists %q", r.Value)
	}
}

func TestFSEditIsUndefined(t *testing.T) {
	v := newVM(t)
	if r := v.Exec(context.Background(), `typeof fs.edit`); r.Err != "" || r.Value != "undefined" {
		t.Fatalf("fs.edit must be undefined: %+v", r)
	}
	if r := v.Exec(context.Background(), `fs.edit("a.txt", "x", "y")`); r.Err == "" {
		t.Fatal("calling fs.edit must throw")
	}
}

func TestShellRunsInRootAndReturnsCode(t *testing.T) {
	v := newVM(t)
	r := v.Exec(context.Background(), `sh("pwd; exit 3").code`)
	if r.Value != "3" {
		t.Fatalf("exit code %+v", r)
	}
}

func TestTimeoutInterrupts(t *testing.T) {
	v := New(Options{Root: t.TempDir(), Timeout: 100 * time.Millisecond})
	start := time.Now()
	r := v.Exec(context.Background(), `while(true){}`)
	if !r.TimedOut || !strings.Contains(r.Err, "timeout") || time.Since(start) > 3*time.Second {
		t.Fatalf("timeout not enforced: %+v", r)
	}
	if r2 := v.Exec(context.Background(), `1+1`); r2.Value != "2" {
		t.Fatalf("vm unusable after timeout: %+v", r2)
	}
}

func TestAbortViaContext(t *testing.T) {
	v := newVM(t)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	r := v.Exec(ctx, `while(true){}`)
	if r.Err == "" {
		t.Fatal("abort did not stop loop")
	}
}

func TestAwaitCodeRuns(t *testing.T) {
	v := newVM(t)
	r := v.Exec(context.Background(), "const x = await Promise.resolve(5); console.log(x * 2)")
	if r.Err != "" || r.Output != "10\n" {
		t.Fatalf("await: %+v", r)
	}
}

func TestSyntaxErrorReported(t *testing.T) {
	v := newVM(t)
	if r := v.Exec(context.Background(), `let = ;`); r.Err == "" {
		t.Fatal("syntax error not reported")
	}
}

func TestGlobMatching(t *testing.T) {
	cases := []struct {
		pat, rel string
		want     bool
	}{
		{"src/*.ts", "src/a.ts", true},
		{"src/*.ts", "src/x/a.ts", false},
		{"**/*.go", "a/b/c.go", true},
		{"**/*.go", "c.go", true},
		{"src/**", "src/x/y", true},
		{"a?c", "abc", true},
		{"a.b", "axb", false},
	}
	for _, c := range cases {
		if got := MatchGlob(c.pat, c.rel); got != c.want {
			t.Errorf("%s vs %s: got %v", c.pat, c.rel, got)
		}
	}
}

func TestPatchRespectsFileRoot(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("abc"), 0o644)
	v := New(Options{Root: root})
	h := v.Exec(context.Background(), `fs.inspect("f.txt").sha256`).Value
	v.Exec(context.Background(), `fs.patch("f.txt", [{oldText:"b",newText:"B"}], {expectedHash:"`+h+`"})`)
	b, _ := os.ReadFile(filepath.Join(root, "f.txt"))
	if string(b) != "aBc" {
		t.Fatalf("edit result %q", b)
	}
}
