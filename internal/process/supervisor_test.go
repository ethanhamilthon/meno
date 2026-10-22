package process

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"meno/internal/db"
)

func newSup(t *testing.T, fin func(Task)) (*Supervisor, string) {
	t.Helper()
	home := t.TempDir()
	d, err := db.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.CloseAll)
	s, err := New(d, home+"/tasks", Hooks{Finished: fin})
	if err != nil {
		t.Fatal(err)
	}
	return s, home
}

func start(t *testing.T, s *Supervisor, session, cmd string, mod func(*StartOptions)) Task {
	t.Helper()
	o := StartOptions{Session: session, Owner: "req1", Command: cmd, Cwd: t.TempDir(), Notify: true}
	if mod != nil {
		mod(&o)
	}
	task, err := s.Start(o)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestSuccessFailureAndLogsKeepStreamsApart(t *testing.T) {
	fin := make(chan Task, 2)
	s, _ := newSup(t, func(t Task) { fin <- t })
	a := start(t, s, "s1", "echo out1; echo err1 >&2; echo out2", nil)
	got, timedOut, err := s.Wait(context.Background(), "s1", a.ID, 10*time.Second)
	if err != nil || timedOut || got.Status != Succeeded || got.ExitCode == nil || *got.ExitCode != 0 {
		t.Fatalf("%+v %v %v", got, timedOut, err)
	}
	logs, _ := s.Logs("s1", a.ID, LogOptions{Tail: true})
	var out, errs []string
	for _, r := range logs.Records {
		if r.Stream == Stdout {
			out = append(out, r.Text)
		} else {
			errs = append(errs, r.Text)
		}
	}
	if strings.Join(out, ",") != "out1,out2" || strings.Join(errs, ",") != "err1" {
		t.Fatalf("%+v", logs.Records)
	}
	if only, _ := s.Logs("s1", a.ID, LogOptions{Tail: true, Stream: Stderr}); len(only.Records) != 1 {
		t.Fatalf("stream filter: %+v", only)
	}
	b := start(t, s, "s1", "echo boom >&2; exit 3", nil)
	got, _, _ = s.Wait(context.Background(), "s1", b.ID, 10*time.Second)
	if got.Status != Failed || got.ExitCode == nil || *got.ExitCode != 3 {
		t.Fatalf("a later nonzero exit is failed task data: %+v", got)
	}
	for range 2 {
		select {
		case <-fin:
		case <-time.After(time.Second):
			t.Fatal("finish hook was not called")
		}
	}
	if _, err := s.Start(StartOptions{Session: "s1", Command: "x", Cwd: "/does/not/exist"}); err == nil {
		t.Fatal("startup validation")
	}
}

func TestCancelKillsTheWholeGroupAndTimeoutDoesNotCancelWait(t *testing.T) {
	SetGraceTime(300 * time.Millisecond)
	s, _ := newSup(t, nil)
	pidFile := t.TempDir() + "/pid"
	// A shell that ignores SIGTERM and has a child: both must die.
	a := start(t, s, "s1", "trap '' TERM; sleep 60 & echo $! > "+pidFile+"; wait", nil)
	time.Sleep(300 * time.Millisecond)
	_, timedOut, _ := s.Wait(context.Background(), "s1", a.ID, 50*time.Millisecond)
	if !timedOut {
		t.Fatal("wait must time out while the task runs")
	}
	if got, _ := s.Get("s1", a.ID); got.Status != Running {
		t.Fatalf("a wait timeout must not cancel: %+v", got)
	}
	if _, err := s.Cancel("s1", a.ID); err != nil {
		t.Fatal(err)
	}
	got, _, _ := s.Wait(context.Background(), "s1", a.ID, 10*time.Second)
	if got.Status != Cancelled {
		t.Fatalf("%+v", got)
	}
	b, _ := os.ReadFile(pidFile)
	var pid int
	fmt.Sscan(string(b), &pid)
	time.Sleep(200 * time.Millisecond)
	if pid == 0 || syscall.Kill(pid, 0) == nil {
		t.Fatalf("the child %d survived the group kill", pid)
	}
	if _, err := s.Cancel("s1", a.ID); err != nil {
		t.Fatal("cancel is idempotent")
	}
	if again, _ := s.Get("s1", a.ID); again.Status != Cancelled {
		t.Fatalf("a finished task is not restarted by cancel: %+v", again)
	}
}

func TestLogsRotateKeepNewestAndReportGaps(t *testing.T) {
	s, _ := newSup(t, nil)
	s.SetLogLimit(16 << 10)
	a := start(t, s, "s1", "i=0; while [ $i -lt 2000 ]; do echo line-$i-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx; i=$((i+1)); done", nil)
	s.Wait(context.Background(), "s1", a.ID, 20*time.Second)
	tail, _ := s.Logs("s1", a.ID, LogOptions{Tail: true, Limit: 5})
	if len(tail.Records) != 5 || !strings.HasPrefix(tail.Records[4].Text, "line-1999-") {
		t.Fatalf("tail: %+v", tail.Records)
	}
	if tail.FirstSeq <= 1 {
		t.Fatalf("old output should have rotated away, first seq %d", tail.FirstSeq)
	}
	// An expired cursor starts at the first available record and says what was lost.
	old, _ := s.Logs("s1", a.ID, LogOptions{Cursor: 3, Limit: 2})
	if old.Lost <= 0 || old.Records[0].Seq != old.FirstSeq || !old.Truncated {
		t.Fatalf("gap: %+v", old)
	}
	// Reading on from a cursor continues without repeats.
	next, _ := s.Logs("s1", a.ID, LogOptions{Cursor: old.NextCursor, Limit: 2})
	if next.Records[0].Seq != old.Records[1].Seq+1 || next.Lost != 0 {
		t.Fatalf("continuation: %+v then %+v", old, next)
	}
	// Byte bound.
	big, _ := s.Logs("s1", a.ID, LogOptions{Tail: true, Bytes: 100})
	if len(big.Records) > 3 {
		t.Fatalf("byte bound ignored: %d records", len(big.Records))
	}
}

func TestSessionIsolationRestartAndShutdown(t *testing.T) {
	s, home := newSup(t, nil)
	a := start(t, s, "s1", "sleep 30", nil)
	if _, err := s.Get("s2", a.ID); err == nil {
		t.Fatal("another session must not see the task")
	}
	if _, err := s.Cancel("s2", a.ID); err == nil {
		t.Fatal("another session must not cancel the task")
	}
	live, _ := s.List("s1", false)
	if len(live) != 1 {
		t.Fatalf("%+v", live)
	}
	SetGraceTime(200 * time.Millisecond)
	s.Shutdown()
	if got, _ := s.Get("s1", a.ID); got.Status != Cancelled {
		t.Fatalf("shutdown cancels: %+v", got)
	}
	// A task found running after a crash is interrupted, never restarted.
	d, _ := db.Open(home)
	d.Exec("UPDATE tasks SET status='running', ended_at='' WHERE id = ?", a.ID)
	s2, err := New(d, home+"/tasks", Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s2.Get("s1", a.ID); got.Status != Interrupted || got.Reason == "" {
		t.Fatalf("%+v", got)
	}
	if _, err := s2.Start(StartOptions{Session: "s1", Command: "true", Cwd: home}); err != nil {
		t.Fatal(err)
	}
	s2.Shutdown()
	if _, err := s2.Start(StartOptions{Session: "s1", Command: "true", Cwd: home}); err == nil {
		t.Fatal("no starts after shutdown")
	}
}
