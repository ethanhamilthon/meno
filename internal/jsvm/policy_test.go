package jsvm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readOnlyVM(t *testing.T) (*VM, string) {
	t.Helper()
	v := newVM(t)
	v.SetPolicy(Policy{ReadOnly: true, Why: "plan mode"})
	return v, v.Root()
}

func TestReadOnlyRefusesWritesButAllowsReads(t *testing.T) {
	v, root := readOnlyVM(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{
		"fs.write('b.txt', 'x')",
		"fs.write('a.txt', 'x', {overwrite: true})",
		"fs.edit('a.txt', 'one', 'two')",
		"fs.patch('a.txt', [{oldText: 'one', newText: 'two'}])",
	} {
		r := ex(t, v, code)
		if !strings.Contains(r.Err, "not available in plan mode") {
			t.Errorf("%s: %+v", code, r)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(b) != "one" {
		t.Fatalf("the file changed: %q", b)
	}
	if _, err := os.Stat(filepath.Join(root, "b.txt")); err == nil {
		t.Fatal("a file was created")
	}
	if r := ex(t, v, "fs.read('a.txt') + fs.glob('*.txt').length + fs.exists('a.txt')"); r.Err != "" || r.Value != "one1true" {
		t.Fatalf("reads must work: %+v", r)
	}
	v.SetPolicy(Policy{})
	if r := ex(t, v, "fs.write('b.txt', 'x'); fs.read('b.txt')"); r.Err != "" || r.Value != "x" {
		t.Fatalf("the policy must be liftable: %+v", r)
	}
}

func TestReadOnlyShellCannotWriteButCanRead(t *testing.T) {
	// The directory must be outside the temporary ones, which stay writable.
	dir, err := os.MkdirTemp(".", "sandbox-*")
	if err != nil {
		t.Fatal(err)
	}
	root, _ := filepath.Abs(dir)
	t.Cleanup(func() { os.RemoveAll(root) })
	v := New(Options{Root: root, Timeout: 5 * time.Second})
	v.SetPolicy(Policy{ReadOnly: true, Why: "plan mode"})
	if _, _, ok := readOnlyCommand("true"); !ok {
		r := ex(t, v, "sh('echo hi')")
		if !strings.Contains(r.Err, "no sandbox") {
			t.Fatalf("without a sandbox sh must refuse, got %+v", r)
		}
		t.Skip("no sandbox tool on this system")
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := ex(t, v, "sh('cat a.txt').stdout")
	if r.Err != "" || strings.TrimSpace(r.Value) != "one" {
		t.Fatalf("reading in the sandbox: %+v", r)
	}
	r = ex(t, v, "var w = sh('echo changed > a.txt; echo more > new.txt'); w.code")
	if r.Err != "" || r.Value == "0" {
		t.Fatalf("a write must fail: %+v", r)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(b) != "one" {
		t.Fatalf("the sandbox let a file change: %q", b)
	}
	if _, err := os.Stat(filepath.Join(root, "new.txt")); err == nil {
		t.Fatal("the sandbox let a file appear")
	}
	if r := ex(t, v, "sh('echo ok > \"$TMPDIR/meno-ro-probe\" && cat \"$TMPDIR/meno-ro-probe\"').stdout"); r.Err != "" || !strings.Contains(r.Value, "ok") {
		t.Fatalf("temporary files must stay writable: %+v", r)
	}
}

type fakeGoal struct{ done, blocked string }

func (g *fakeGoal) Done(s string) error    { g.done = s; return nil }
func (g *fakeGoal) Blocked(s string) error { g.blocked = s; return nil }

func TestGoalGlobal(t *testing.T) {
	v := newVM(t)
	if r := ex(t, v, "goal.done('x')"); !strings.Contains(r.Err, "only available in a #goal run") {
		t.Fatalf("%+v", r)
	}
	g := &fakeGoal{}
	v.SetGoal(g)
	if r := ex(t, v, "goal.done('all green'); goal.blocked('no key')"); r.Err != "" || g.done != "all green" || g.blocked != "no key" {
		t.Fatalf("%+v %+v", r, g)
	}
	if r := ex(t, v, "goal.done()"); r.Err == "" {
		t.Fatal("a summary is required")
	}
	v.SetGoal(nil)
	if r := ex(t, v, "goal.done('x')"); r.Err == "" {
		t.Fatal("goal off again")
	}
}
