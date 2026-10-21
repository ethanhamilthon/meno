package client

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestEnsureStartsSingleDaemon(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "meno")
	build := exec.Command("go", "build", "-o", bin, "./cmd/meno")
	build.Dir = "../.."
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			_, e := Ensure(ctx, Options{Home: home, Executable: bin})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	// Keep daemon cleanup in this test's scope: a nested subtest would kill
	// the replacement before the parent verifies its PID and shuts it down.
	testUpdateNextLaunch(t, home, bin)
	data, e := os.ReadFile(filepath.Join(home, "run", "daemon.pid"))
	if e != nil {
		t.Fatal(e)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGTERM) })
	if e := New(home).Shutdown(context.Background(), "test"); e != nil {
		t.Logf("shutdown error: %v", e)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e := syscall.Kill(pid, 0); e != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
}
func TestEnsureColdStartGate(t *testing.T) {
	start := time.Now()
	_ = New(t.TempDir())
	if time.Since(start) > time.Second {
		t.Fatal("cold client construction exceeded one second")
	}
}
