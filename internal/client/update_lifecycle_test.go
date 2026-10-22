package client

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/daemon"
	"meno/internal/db"
	"meno/internal/llm"
)

// Uses the real executable built by TestEnsureStartsSingleDaemon. Release HTTP
// and checksum handling are tested separately in update; this covers the live
// process boundary, binary replacement and next-launch session recovery.
func testUpdateNextLaunch(t *testing.T, home, bin string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	old, err := New(home).Hello(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(old.PID, syscall.SIGTERM) })
	saved := agent.NewSession(home, "fake")
	saved.Append(agent.Turn{Role: llm.RoleUser, Text: "survives executable update"})
	if err := agent.SaveSession(home, saved); err != nil {
		t.Fatal(err)
	}
	defer db.CloseAll()
	transition, err := daemon.AcquireTransition(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	defer transition.Close()
	next := bin + ".new"
	src, err := os.Open(bin)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.OpenFile(next, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(dst, src); err != nil {
		dst.Close()
		t.Fatal(err)
	}
	dst.Close()
	ownership, err := StopConfirmed(ctx, home, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ownership.Close()
	if _, err := New(home).Hello(ctx); err == nil {
		t.Fatal("old daemon still reachable")
	}
	if _, err := os.Stat(filepath.Join(home, "run", "daemon.pid")); !os.IsNotExist(err) {
		t.Fatalf("old daemon not cleaned up: %v", err)
	}
	if err := os.Rename(next, bin); err != nil {
		t.Fatal(err)
	}
	if _, err := New(home).Hello(ctx); err == nil {
		t.Fatal("replacement started before next launch")
	}
	// A reconnect attempt cannot pass the updater's transition lock.
	blocked, cancelBlocked := context.WithTimeout(ctx, 35*time.Millisecond)
	if _, err := Ensure(blocked, Options{Home: home, Executable: bin}); err == nil {
		t.Fatal("reconnect started daemon during update")
	}
	cancelBlocked()
	ownership.Close()
	transition.Close()
	c, err := Ensure(ctx, Options{Home: home, Executable: bin})
	if err != nil {
		log, _ := os.ReadFile(filepath.Join(home, "log", "daemon.log"))
		t.Fatalf("next launch: %v; daemon log:\n%s", err, log)
	}
	now, err := c.Hello(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(now.PID, syscall.SIGTERM) })
	if now.BootID == old.BootID || now.PID == old.PID {
		t.Fatalf("daemon not replaced: old %v new %v", old, now)
	}
	sessions, err := c.ListSessions(ctx, api.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range sessions.Sessions {
		if it.ID == saved.ID {
			return
		}
	}
	t.Fatal("persisted session missing after next launch")
}
