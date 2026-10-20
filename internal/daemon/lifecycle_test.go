package daemon

import (
	"context"
	"os"
	"testing"
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
