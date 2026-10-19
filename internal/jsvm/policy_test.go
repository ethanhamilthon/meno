package jsvm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyRemovesToolGlobals(t *testing.T) {
	v := newVM(t)
	v.SetPolicy(Policy{Only: []string{"agents", "goal", "askUser", "help", "hash"}})
	r := ex(t, v, "[typeof fs, typeof sh, typeof view, typeof tasks, typeof mcp, typeof hash, typeof Math].join(',')")
	if r.Err != "" || r.Value != "undefined,undefined,undefined,undefined,undefined,function,object" {
		t.Fatalf("%+v", r)
	}
	if r := ex(t, v, "fs.read('x')"); r.Err == "" {
		t.Fatalf("a removed global must fail as an unknown name: %+v", r)
	}
}

func TestPolicyNilKeepsEverything(t *testing.T) {
	v := newVM(t)
	v.SetPolicy(Policy{})
	if r := ex(t, v, "typeof fs + typeof sh + typeof tasks"); r.Err != "" || r.Value != "objectfunctionundefined" {
		t.Fatalf("%+v", r)
	}
}

func TestPolicyAppliesToTheFreshRuntime(t *testing.T) {
	v := newVM(t)
	v.SetPolicy(Policy{Only: []string{"hash"}})
	ex(t, v, "var x = 1")
	v.Reset()
	if r := ex(t, v, "typeof fs"); r.Err != "" || r.Value != "undefined" {
		t.Fatalf("the restriction must hold after Reset: %+v", r)
	}
	v.SetPolicy(Policy{})
	v.Reset()
	if r := ex(t, v, "fs.write('p-test.txt', 'x'); fs.read('p-test.txt')"); r.Err != "" || r.Value != "x" {
		t.Fatalf("lifting the policy must restore fs: %+v", r)
	}
}

func TestShellWorksWithoutAPolicy(t *testing.T) {
	dir := t.TempDir()
	v := New(Options{Root: dir})
	if r := ex(t, v, "sh('echo ok > a.txt; cat a.txt').stdout"); r.Err != "" || strings.TrimSpace(r.Value) != "ok" {
		t.Fatalf("%+v", r)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); strings.TrimSpace(string(b)) != "ok" {
		t.Fatalf("the file: %q", b)
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
