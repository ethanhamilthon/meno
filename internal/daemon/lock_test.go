package daemon

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

func TestLockInheritSerialization(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	a, e := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if e = unix.Flock(int(a.Fd()), unix.LOCK_EX); e != nil {
		t.Fatal(e)
	}
	b, e := os.OpenFile(p, os.O_RDWR, 0o600)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	if e = unix.Flock(int(b.Fd()), unix.LOCK_EX|unix.LOCK_NB); e == nil {
		t.Fatal("second lock unexpectedly acquired")
	}
	_ = unix.Flock(int(a.Fd()), unix.LOCK_UN)
	if e = unix.Flock(int(a.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		t.Fatal(e)
	}
}
