package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"meno/internal/api"
)

// A session has one owner process. The engine takes an exclusive lock on
// run/lock-<id> when it opens the session and keeps it until the session is
// deleted or the engine closes. Lock files are never removed. The lock belongs to the open file, so the
// system drops it when the process dies, however it dies.

// lockSession takes the lock of a session; opening it twice is fine.
func (e *Engine) lockSession(id string) error {
	e.lockMu.Lock()
	defer e.lockMu.Unlock()
	if e.locks[id] != nil {
		return nil
	}
	dir := filepath.Join(e.home, "run")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "lock-"+id), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return dapi(api.CodeConflict, fmt.Sprintf("session %s is open in another meno process (a window, meno -p or an ACP connection). Close it there first", id))
	}
	e.locks[id] = f
	return nil
}

// unlockSession gives the session up. The lock file stays: removing it while
// another process is about to lock it would let two processes hold "the" lock
// on two different files. Closing the file drops the lock.
func (e *Engine) unlockSession(id string) {
	e.lockMu.Lock()
	defer e.lockMu.Unlock()
	if f := e.locks[id]; f != nil {
		f.Close()
		delete(e.locks, id)
	}
}

func (e *Engine) unlockAll() {
	e.lockMu.Lock()
	defer e.lockMu.Unlock()
	for id, f := range e.locks {
		f.Close()
		delete(e.locks, id)
	}
}
