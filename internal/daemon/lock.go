package daemon

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func lockFile(runDir string) (*os.File, error) {
	if fd, err := strconv.Atoi(os.Getenv("MENO_DAEMON_LOCK_FD")); err == nil && fd >= 3 {
		f := os.NewFile(uintptr(fd), "daemon.lock")
		if f != nil {
			if _, e := f.Stat(); e == nil {
				path := filepath.Join(runDir, "daemon.lock")
				if named, e := os.Open(path); e == nil {
					_ = f.Close()
					return named, nil
				}
				return f, nil
			}
		}
	}
	return os.OpenFile(filepath.Join(runDir, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0o600)
}
func acquireLock(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
func unlockFile(f *os.File) error  { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
func waitForInheritedLock(f *os.File, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if e := acquireLock(f); e == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrAlreadyRunning
		}
		time.Sleep(10 * time.Millisecond)
	}
}
