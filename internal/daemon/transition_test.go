package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTransitionSerializesUpdatesAndStartup(t *testing.T) {
	home := t.TempDir()
	lock, err := AcquireTransition(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()
	if other, err := AcquireTransition(ctx, home); err == nil {
		other.Close()
		t.Fatal("competing update acquired transition")
	}
	started := make(chan *os.File, 1)
	go func() { f, _ := StartupOwnership(home); started <- f }()
	select {
	case f := <-started:
		if f != nil {
			f.Close()
		}
		t.Fatal("startup crossed update")
	case <-time.After(30 * time.Millisecond):
	}
	lock.Close()
	select {
	case f := <-started:
		if f == nil {
			t.Fatal("startup failed")
		}
		f.Close()
	case <-time.After(time.Second):
		t.Fatal("startup remained blocked")
	}
}

func TestExecutableReplacementRequiresReopen(t *testing.T) {
	// Substitute only the identity fixture, never the running test executable.
	oldPath, oldInfo := processExecutable, processExecutableInfo
	defer func() { processExecutable, processExecutableInfo = oldPath, oldInfo }()
	processExecutable = filepath.Join(t.TempDir(), "meno")
	os.WriteFile(processExecutable, []byte("old"), 0755)
	processExecutableInfo, _ = os.Stat(processExecutable)
	if err := CheckExecutable(); err != nil {
		t.Fatal(err)
	}
	next := processExecutable + ".new"
	os.WriteFile(next, []byte("new"), 0755)
	os.Rename(next, processExecutable)
	if err := CheckExecutable(); err == nil {
		t.Fatal("old client allowed to start daemon")
	}
}
