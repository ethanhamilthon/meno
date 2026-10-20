package proc

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func launch(t *testing.T, s *Supervisor, cmd string) *Foreground {
	t.Helper()
	dir := t.TempDir()
	f, err := s.Launch(LaunchOptions{Session: "s1", Owner: "req", Command: cmd, Cwd: dir, Env: []string{"PATH=/usr/bin:/bin"}, CallID: "c" + newID(), OutputDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestForegroundIsPrivateUntilPromoted(t *testing.T) {
	var mu sync.Mutex
	var finished []Task
	s, _ := newSup(t, func(task Task) { mu.Lock(); finished = append(finished, task); mu.Unlock() })
	f := launch(t, s, "echo hi")
	<-f.Done()
	res, _ := f.Result()
	if !strings.Contains(res.Text, "hi") {
		t.Fatalf("%q", res.Text)
	}
	if list, _ := s.List("s1", true); len(list) != 0 {
		t.Fatalf("a foreground call is not a task: %+v", list)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(finished) != 0 {
		t.Fatalf("no finished notice for a foreground call: %+v", finished)
	}
	if _, ok := f.Promote(); ok {
		t.Fatal("a finished call cannot be promoted")
	}
}

func TestPromotionKeepsTheSameProcessAndLog(t *testing.T) {
	var mu sync.Mutex
	var finished []Task
	s, _ := newSup(t, func(task Task) { mu.Lock(); finished = append(finished, task); mu.Unlock() })
	f := launch(t, s, "echo before; sleep 0.5; echo after")
	time.Sleep(150 * time.Millisecond)
	pid := f.r.call.Pid()
	task, ok := f.Promote()
	again, ok2 := f.Promote()
	if !ok || !ok2 || task.ID != again.ID || task.ID != f.TaskID() {
		t.Fatalf("promotion must be idempotent: %+v %+v", task, again)
	}
	f.Cancel() // the request ends: a promoted process must survive
	got, to, err := s.Wait(context.Background(), "s1", task.ID, 10*time.Second)
	if err != nil || to || got.Status != Succeeded {
		t.Fatalf("%+v %v %v", got, to, err)
	}
	if f.r.call.Pid() != pid {
		t.Fatal("the process changed")
	}
	logs, _ := s.Logs("s1", task.ID, LogOptions{Tail: true})
	var text string
	for _, r := range logs.Records {
		text += r.Text + "|"
	}
	if text != "before|after|" {
		t.Fatalf("log must run on through promotion: %q", text)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(finished) != 1 || finished[0].ID != task.ID {
		t.Fatalf("exactly one completion notice: %+v", finished)
	}
}

func TestFinishVersusPromoteHasOneWinner(t *testing.T) {
	var mu sync.Mutex
	notices := 0
	s, _ := newSup(t, func(Task) { mu.Lock(); notices++; mu.Unlock() })
	promoted := 0
	for i := 0; i < 30; i++ {
		f := launch(t, s, "true")
		time.Sleep(time.Duration(i) * time.Millisecond)
		_, ok := f.Promote()
		<-f.Done()
		if ok {
			promoted++
		}
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if notices != promoted {
		t.Fatalf("%d promotions but %d notices", promoted, notices)
	}
}

func TestForegroundCancelKillsUnpromotedAndTasksSurvive(t *testing.T) {
	s, _ := newSup(t, nil)
	task := start(t, s, "s1", "sleep 30", nil)
	f := launch(t, s, "sleep 30")
	f.Cancel()
	select {
	case <-f.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop the foreground call")
	}
	if got, _ := s.Get("s1", task.ID); got.Status != Running {
		t.Fatalf("Stop must not touch tasks: %+v", got)
	}
	SetGraceTime(200 * time.Millisecond)
	s.Shutdown()
}

func TestShutdownStopsPromotedProcess(t *testing.T) {
	s, _ := newSup(t, nil)
	f := launch(t, s, "sleep 30")
	task, _ := f.Promote()
	SetGraceTime(200 * time.Millisecond)
	s.Shutdown()
	if got, _ := s.Get("s1", task.ID); got.Status != Cancelled {
		t.Fatalf("%+v", got)
	}
}
