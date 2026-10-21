package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// AcquireTransition serializes updates and daemon startup without taking the
// daemon's lifetime lock. Always acquire this before daemon.lock.
func AcquireTransition(ctx context.Context, home string) (*os.File, error) {
	return AcquireOwnership(ctx, home, "transition.lock")
}

// AcquireOwnership waits for a per-home lifecycle lock. Closing releases it.
func AcquireOwnership(ctx context.Context, home, name string) (*os.File, error) {
	dir := filepath.Join(home, "run")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		if err = ctx.Err(); err != nil {
			f.Close()
			return nil, err
		}
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

var processExecutable, processExecutableInfo = executableIdentity()

func executableIdentity() (string, os.FileInfo) {
	path, _ := os.Executable()
	info, _ := os.Stat(path)
	return path, info
}

// CheckExecutable prevents an already-open old client from spawning its old
// executable after replacement. Users must reopen the application instead.
func CheckExecutable() error {
	if processExecutableInfo == nil {
		return nil
	}
	now, err := os.Stat(processExecutable)
	if err != nil || !os.SameFile(processExecutableInfo, now) {
		return fmt.Errorf("meno executable changed; close and reopen the application")
	}
	return nil
}
