package daemon

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestServeLeavesRegularSocketPathUntouched(t *testing.T) {
	home := t.TempDir()
	s, e := New(Options{Home: home})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(s.Socket(), []byte("regular file"), 0o600); e != nil {
		t.Fatal(e)
	}
	if e = s.Serve(context.Background()); e == nil {
		t.Fatal("server overwrote non-socket")
	}
	b, e := os.ReadFile(s.Socket())
	if e != nil || string(b) != "regular file" {
		t.Fatalf("regular path changed: %q %v", b, e)
	}
}
func TestSecondDaemonLock(t *testing.T) {
	home := t.TempDir()
	first, _ := New(Options{Home: home})
	f, e := lockFile(first.runDir)
	if e != nil {
		t.Fatal(e)
	}
	if e = acquireLock(f); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = unlockFile(f); _ = f.Close() }()
	second, _ := New(Options{Home: home})
	if e = second.Serve(context.Background()); e != ErrAlreadyRunning {
		t.Fatalf("got %v", e)
	}
}

func TestStopBeforeServeCannotStartLateListener(t *testing.T) {
	s, e := New(Options{Home: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	s.Stop("before startup")
	done := make(chan error, 1)
	go func() { done <- s.Serve(context.Background()) }()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("late Serve did not stop")
	}
	if _, e := os.Stat(s.Socket()); !os.IsNotExist(e) {
		t.Fatalf("late socket remains: %v", e)
	}
}

func TestStopDuringStartupClosesListener(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	s, e := New(Options{Home: t.TempDir(), OnStart: func(context.Context) error { close(entered); <-release; return nil }})
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(context.Background()) }()
	<-entered
	stopped := make(chan struct{})
	go func() { s.Stop("during startup"); close(stopped) }()
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop stalled")
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve survived shutdown")
	}
}
