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

func TestGlobalsPersistAcrossExec(t *testing.T) {
	v := newVM(t)
	v.Exec(context.Background(), "var n = 2")
	r := v.Exec(context.Background(), "n * 21")
	if r.Err != "" || r.Value != "42" {
		t.Fatalf("got %+v", r)
	}
}

func TestPrintCaptured(t *testing.T) {
	v := newVM(t)
	r := v.Exec(context.Background(), `console.log("a", 1, true)`)
	if r.Output != "a 1 true\n" {
		t.Fatalf("output %q", r.Output)
	}
}

func TestFSWriteReadEditGlob(t *testing.T) {
	v := newVM(t)
	ctx := context.Background()
	r := v.Exec(ctx, `fs.write("src/a.ts", "hello rateLimit"); fs.write("src/b.ts", "x")`)
	if r.Err != "" {
		t.Fatal(r.Err)
	}
	r = v.Exec(ctx, `fs.edit("src/a.ts", "hello", "bye")`)
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

func TestEditErrors(t *testing.T) {
	v := newVM(t)
	ctx := context.Background()
	v.Exec(ctx, `fs.write("a.txt", "x x")`)
	if r := v.Exec(ctx, `fs.edit("a.txt", "x", "y")`); !strings.Contains(r.Err, "matches 2") {
		t.Fatalf("ambiguous edit should fail: %+v", r)
	}
	if r := v.Exec(ctx, `fs.edit("a.txt", "zzz", "y")`); !strings.Contains(r.Err, "not found") {
		t.Fatalf("missing edit should fail: %+v", r)
	}
	if r := v.Exec(ctx, `fs.edit("a.txt", "", "y")`); r.Err == "" {
		t.Fatal("empty old should fail")
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

func TestResetClearsGlobals(t *testing.T) {
	v := newVM(t)
	v.Exec(context.Background(), `var keep = 7`)
	v.Reset()
	if r := v.Exec(context.Background(), `typeof keep`); r.Value != "undefined" {
		t.Fatalf("reset kept globals: %s", r.Value)
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

func TestEditRespectsFileRoot(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("abc"), 0o644)
	v := New(Options{Root: root})
	v.Exec(context.Background(), `fs.edit("f.txt","b","B")`)
	b, _ := os.ReadFile(filepath.Join(root, "f.txt"))
	if string(b) != "aBc" {
		t.Fatalf("edit result %q", b)
	}
}
