package proc

import (
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if ok() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("timeout: " + what)
}

func TestDaemonStartsOnceLogsAndStopsTheWholeGroup(t *testing.T) {
	dir := t.TempDir()
	d := NewDaemons(dir)
	pidFile := dir + "/child.pid"
	cmd := "echo hello; sleep 60 & echo $! > " + pidFile + "; wait"
	if err := d.Start("p/1", cmd, dir, os.Environ()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "child pid", func() bool { b, _ := os.ReadFile(pidFile); return strings.TrimSpace(string(b)) != "" })
	if !d.Running("p/1") {
		t.Fatal("not running")
	}
	if err := d.Start("p/1", cmd, dir, os.Environ()); err != nil { // the same command: no restart
		t.Fatal(err)
	}
	b, _ := os.ReadFile(pidFile)
	var pid int
	for _, c := range strings.TrimSpace(string(b)) {
		pid = pid*10 + int(c-'0')
	}
	if out, _ := os.ReadFile(d.LogPath("p/1")); !strings.Contains(string(out), "hello") {
		t.Fatalf("log: %q", out)
	}
	d.StopAll()
	if d.Running("p/1") {
		t.Fatal("still running")
	}
	waitFor(t, "the child to die with its group", func() bool { return syscall.Kill(pid, 0) != nil })
}

func TestDaemonThatEndsByItselfIsReported(t *testing.T) {
	dir := t.TempDir()
	d := NewDaemons(dir)
	got := make(chan error, 1)
	d.OnExit = func(name string, err error, log string) { got <- err }
	if err := d.Start("bad", "echo boom >&2; exit 3", dir, os.Environ()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-got:
		if err == nil || !strings.Contains(err.Error(), "exit status 3") {
			t.Fatalf("%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no report")
	}
	if out, _ := os.ReadFile(d.LogPath("bad")); !strings.Contains(string(out), "boom") {
		t.Fatalf("log: %q", out)
	}
}

func TestStoppingADaemonIsNotAnExitReport(t *testing.T) {
	dir := t.TempDir()
	d := NewDaemons(dir)
	d.OnExit = func(string, error, string) { t.Error("a stop is not a failure") }
	if err := d.Start("x", "sleep 60", dir, os.Environ()); err != nil {
		t.Fatal(err)
	}
	d.Stop("x")
	time.Sleep(100 * time.Millisecond)
	if err := d.Start("y", "", dir, nil); err != nil { // an empty command stops and starts nothing
		t.Fatal(err)
	}
}

func TestChangedCommandReplacesTheDaemon(t *testing.T) {
	dir := t.TempDir()
	d := NewDaemons(dir)
	defer d.StopAll()
	_ = d.Start("x", "echo one; sleep 60", dir, os.Environ())
	waitFor(t, "first log", func() bool { b, _ := os.ReadFile(d.LogPath("x")); return strings.Contains(string(b), "one") })
	_ = d.Start("x", "echo two; sleep 60", dir, os.Environ())
	waitFor(t, "second log", func() bool { b, _ := os.ReadFile(d.LogPath("x")); return strings.Contains(string(b), "two") })
	if b, _ := os.ReadFile(d.LogPath("x")); strings.Contains(string(b), "one") {
		t.Fatal("the log was not restarted")
	}
}
