package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// A finished task wakes the idle session once, with the task in the event.
func TestTaskCompletionWakesIdleSession(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "build it")
	id := execJS(t, a, "tasks.start('echo built; exit 2', {name: 'build'}).id", nil)
	if !strings.HasPrefix(id, "t") {
		t.Fatalf("start: %q", id)
	}
	pump(t, a, func() bool { return len(cl.requests()) == 2 && !a.st.Running })
	got := lastUser(cl.requests()[1])
	if !strings.Contains(got, "source=\"task\"") || !strings.Contains(got, "failed, exit code 2") || !strings.Contains(got, id) {
		t.Fatalf("event: %q", got)
	}
	logs := execJS(t, a, "tasks.logs('"+id+"').records[0].text", nil)
	if logs != "built" {
		t.Fatalf("logs: %q", logs)
	}
	if st := execJS(t, a, "tasks.status('"+id+"').status + ':' + tasks.status('"+id+"').exitCode", nil); st != "failed:2" {
		t.Fatalf("status: %q", st)
	}
}

// An explicit successful wait acknowledges the completion, so the model is
// not told twice; a notify:false task never wakes the session.
func TestWaitAcknowledgesAndNotifyFalseStaysQuiet(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "go")
	got := execJS(t, a, "const t = tasks.start('echo hi'); const r = tasks.wait(t.id, {timeout: 10}); r.status + ':' + r.waitTimedOut", nil)
	if got != "succeeded:false" {
		t.Fatalf("wait: %q", got)
	}
	got = execJS(t, a, "const q = tasks.start('echo quiet', {notify: false}); tasks.wait(q.id, {timeout: 10}).status", nil)
	if got != "succeeded" {
		t.Fatalf("quiet: %q", got)
	}
	pump(t, a, func() bool { return true })
	if len(cl.requests()) != 1 {
		t.Fatalf("a waited or silent task woke the session: %d requests", len(cl.requests()))
	}
	// Children and unknown ids: unauthorized IDs throw.
	if got := execJS(t, a, "tasks.status('tnope')", nil); !strings.Contains(got, "not found") {
		t.Fatalf("unknown id: %q", got)
	}
	if got := execJS(t, a, "tasks.start('x', {cwd: '/definitely/not/here'})", nil); !strings.Contains(got, "not a directory") {
		t.Fatalf("cwd validation: %q", got)
	}
}

// :stop cancels the tasks of the active request only.
func TestStopCancelsOnlyTheActiveRequestsTasks(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "first request")
	old := execJS(t, a, "tasks.start('sleep 30').id", nil)
	say(t, a, "second request")
	cur := execJS(t, a, "tasks.start('sleep 30').id", nil)
	a.abortTurn()
	for i := 0; i < 100; i++ {
		if t1, _ := a.tasks.Get(a.st.Session.ID, cur); t1.Terminal() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if t1, _ := a.tasks.Get(a.st.Session.ID, cur); t1.Status != "cancelled" {
		t.Fatalf("the active request's task: %+v", t1)
	}
	t0, _ := a.tasks.Get(a.st.Session.ID, old)
	if t0.Terminal() {
		t.Fatalf("a task of an earlier request must keep running: %+v", t0)
	}
	a.tasks.Cancel(a.st.Session.ID, old)
}

func TestTasksDialogListsLogsAndCancels(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "go")
	id := execJS(t, a, "tasks.start('echo hello-from-task; sleep 30', {name: 'srv'}).id", nil)
	a.runCommand("tasks")
	for i := 0; i < 100 && len(a.st.TasksUI.Rows) == 0; i++ {
		time.Sleep(20 * time.Millisecond)
		select {
		case e := <-a.events:
			a.handle(e)
		default:
		}
	}
	a.draw()
	text := screenText(scr)
	if !strings.Contains(text, "srv") || !strings.Contains(text, "echo hello-from") || !strings.Contains(text, "running") {
		t.Fatalf("list:\n%s", text)
	}
	if strings.Contains(text, "FOO=") {
		t.Fatal("the environment must not be shown")
	}
	a.onKey(keyK(tcell.KeyEnter)) // open the log
	time.Sleep(200 * time.Millisecond)
	for len(a.events) > 0 {
		a.handle(<-a.events)
	}
	a.refreshTasks()
	for i := 0; i < 100 && len(a.st.TasksUI.Logs.Records) == 0; i++ {
		time.Sleep(20 * time.Millisecond)
		select {
		case e := <-a.events:
			a.handle(e)
		default:
		}
	}
	a.draw()
	if !strings.Contains(screenText(scr), "hello-from-task") {
		t.Fatalf("log:\n%s", screenText(scr))
	}
	a.onKey(keyK(tcell.KeyEsc)) // back to the list
	a.onKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	for i := 0; i < 100; i++ {
		if tk, _ := a.tasks.Get(a.st.Session.ID, id); tk.Terminal() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if tk, _ := a.tasks.Get(a.st.Session.ID, id); tk.Status != "cancelled" {
		t.Fatalf("c must cancel the selected task: %+v", tk)
	}
}
