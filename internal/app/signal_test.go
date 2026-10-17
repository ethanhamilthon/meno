package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
)

type failingClient struct{}

func (failingClient) Stream(context.Context, llm.Request, func(llm.Event)) error {
	return errors.New("boom")
}

type signals struct {
	events []string
	cmds   []string
	envs   [][]string
}

func (s *signals) record(cmd string, env []string) {
	s.cmds = append(s.cmds, cmd)
	s.envs = append(s.envs, env)
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "MENO_NOTIFY_EVENT="); ok {
			s.events = append(s.events, v)
		}
	}
}

func runTurn(t *testing.T, a *App, text string) {
	t.Helper()
	typeRunes(a, text)
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool { return !a.st.Running })
}

func TestSignalOnlyForQuestionsInQuestionMode(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "hi"}, {Kind: llm.EvDone}}}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	var s signals
	a.notifier = s.record
	a.st.Settings.Notify = "question"
	runTurn(t, a, "hello")
	if len(s.events) != 0 {
		t.Fatalf("signals: %v", s.events)
	}
}

func TestSignalWhenTurnEnds(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "hi"}, {Kind: llm.EvDone}}}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	var s signals
	a.notifier = s.record
	a.st.Settings.Notify = "done"
	a.st.Settings.NotifyCommand = "mycmd"
	runTurn(t, a, "hello")
	if len(s.events) != 1 || s.events[0] != "done" || s.cmds[0] != "mycmd" {
		t.Fatalf("signals: %v %v", s.events, s.cmds)
	}
	var hasSession bool
	for _, e := range s.envs[0] {
		hasSession = hasSession || strings.HasPrefix(e, "MENO_NOTIFY_SESSION="+a.st.Session.ID)
	}
	if !hasSession {
		t.Fatalf("env: %v", s.envs[0])
	}
}

func TestSignalOnErrorButNotOnAbort(t *testing.T) {
	a, scr, _ := e2eApp(t, failingClient{})
	defer scr.Fini()
	a.retry = agent.RetryPolicy{Attempts: 1}
	var s signals
	a.notifier = s.record
	a.st.Settings.Notify = "done"
	runTurn(t, a, "hello")
	if len(s.events) != 1 || s.events[0] != "error" {
		t.Fatalf("signals after error: %v", s.events)
	}
	s.events = nil
	a.st.Running = true
	a.onTurnDone(context.Canceled)
	if len(s.events) != 0 {
		t.Fatalf("abort signalled: %v", s.events)
	}
}

func TestSignalNotSentWhenQueuedMessageRestartsTurn(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvText, Text: "one"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "two"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	var s signals
	a.notifier = s.record
	a.st.Settings.Notify = "done"
	a.st.Queue.Push(agent.Message{Text: "later"})
	a.st.Running = true
	a.onTurnDone(nil)
	pump(t, a, func() bool { return !a.st.Running })
	if len(s.events) != 1 {
		t.Fatalf("signals: %v", s.events)
	}
}

func TestNotifyCommand(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	var s signals
	a.notifier = s.record
	a.runCommand("notify done")
	a.runCommand("notify command say finished")
	if a.st.Settings.Notify != "done" || a.st.Settings.NotifyCommand != "say finished" {
		t.Fatalf("%+v", a.st.Settings)
	}
	a.runCommand("notify test")
	a.runCommand("notify off")
	a.runCommand("notify test question")
	if len(s.events) != 2 || s.events[0] != "done" || s.events[1] != "question" || s.cmds[0] != "say finished" {
		t.Fatalf("signals: %v %v", s.events, s.cmds)
	}
	if a.st.Settings.Notify != "off" {
		t.Fatal("test must not change the mode")
	}
	a.runCommand("notify loud")
	if !strings.Contains(a.lastNotice(), "usage") {
		t.Fatalf("notice: %q", a.lastNotice())
	}
}

func TestDefaultNotifyCommand(t *testing.T) {
	if !strings.HasPrefix(defaultNotifyCommand("darwin"), "afplay ") || defaultNotifyCommand("linux") != "" {
		t.Fatal("defaults")
	}
	if !wantSignal("done", "error") || wantSignal("question", "done") || !wantSignal("question", "question") || wantSignal("off", "question") {
		t.Fatal("wantSignal")
	}
}
