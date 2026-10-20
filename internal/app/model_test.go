package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestModelChangeProducesEventAndKeepsPrompt(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	a.setSelection("openai", "gpt-6", "low")
	say(t, a, "two")
	say(t, a, "three")
	reqs := cl.requests()
	if len(reqs) != 3 {
		t.Fatalf("%d requests", len(reqs))
	}
	if reqs[0].Model != "gpt-5.5" || reqs[1].Model != "gpt-6" || reqs[1].Effort != "low" {
		t.Fatalf("models %q %q effort %q", reqs[0].Model, reqs[1].Model, reqs[1].Effort)
	}
	if !strings.Contains(reqs[0].System, "\n- Model: gpt-5.5 (openai, effort high).") {
		t.Fatalf("the prompt must name the model:\n%s", reqs[0].System)
	}
	if strings.Contains(lastUser(reqs[0]), "<system-event>") {
		t.Fatal("no event at a reset point")
	}
}

func TestModelBeforeFirstMessageNeedsNoEvent(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.setSelection("openai", "gpt-6", "low")
	say(t, a, "one")
	r := cl.requests()[0]
	if r.Model != "gpt-6" || strings.Contains(lastUser(r), "<system-event>") || !strings.Contains(r.System, "Model: gpt-6 (openai, effort low)") {
		t.Fatalf("%q\n%s", r.Model, r.System)
	}
}

func TestEachSessionKeepsItsOwnModel(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	first := a.st.Runtime
	a.runCommand("new")
	if _, m, _ := a.selection(); m != "gpt-5.5" {
		t.Fatalf("a new session starts with the default model, got %q", m)
	}
	a.setSelection("openai", "gpt-6", "low")
	say(t, a, "two")
	a.switchTo(first)
	if _, m, e := a.selection(); m != "gpt-5.5" || e != "high" {
		t.Fatalf("first session changed to %q (%s)", m, e)
	}
	if !strings.Contains(a.modelLabel(), "gpt-5.5") {
		t.Fatal(a.modelLabel())
	}
	say(t, a, "three")
	reqs := cl.requests()
	if reqs[1].Model != "gpt-6" || reqs[2].Model != "gpt-5.5" {
		t.Fatalf("%q %q", reqs[1].Model, reqs[2].Model)
	}
	if strings.Contains(lastUser(reqs[2]), "<model-changed>") {
		t.Fatal("the first session never changed its model")
	}
}

func TestModelOfRunningTurnIsNotChanged(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	typeRunes(a, "one")
	a.onKey(keyK(tcell.KeyEnter))
	a.setSelection("openai", "gpt-6", "low") // while the turn is starting
	pump(t, a, func() bool { return runDone(a) })
	if got := cl.requests()[0].Model; got != "gpt-5.5" {
		t.Fatalf("the turn must keep the model it started with, got %q", got)
	}
}

func TestSessionsDialogShowsModelOfSelectedRowOnly(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	a.renameSession("one")
	a.running.Wait()
	a.runCommand("new")
	settle(t, a)
	a.setSelection("openai", "gpt-6", "low")
	say(t, a, "two") // runs on gpt-6
	a.renameSession("two")
	a.running.Wait()
	pumpFor(a, 100*time.Millisecond)
	a.openSessions()
	pump(t, a, func() bool { return a.dbRows != nil })
	a.refreshSession(a.st.Session.ID())
	a.running.Wait()
	pumpFor(a, 100*time.Millisecond)
	rows := a.sessionRows()
	models := map[string]string{}
	for _, r := range rows {
		models[r.Title] = r.Model
	}
	if models["one"] != "gpt-5.5 (openai, effort high)" || models["two"] != "gpt-6 (openai, effort low)" {
		t.Fatalf("%v", models)
	}
	a.draw()
	text := screenText(scr)
	if n := strings.Count(text, "effort"); n != 1 {
		t.Fatalf("the model is shown for the selected row only, found %d times:\n%s", n, text)
	}
}
