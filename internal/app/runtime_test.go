package app

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

// gateClient streams "part-<msg>", waits until the test opens the gate of that
// message, then finishes with "done-<msg>" and a usage proportional to its length.
type gateClient struct {
	mu    sync.Mutex
	gates map[string]chan struct{}
}

func newGateClient() *gateClient { return &gateClient{gates: map[string]chan struct{}{}} }

func (c *gateClient) gate(key string) chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gates[key] == nil {
		c.gates[key] = make(chan struct{})
	}
	return c.gates[key]
}

func (c *gateClient) open(key string) { close(c.gate(key)) }

func (c *gateClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	text := lastUser(req)
	if i := strings.LastIndex(text, "\n\n"); i >= 0 {
		text = text[i+2:]
	}
	emit(llm.Event{Kind: llm.EvText, Text: "part-" + text})
	select {
	case <-c.gate(text):
	case <-ctx.Done():
		return ctx.Err()
	}
	emit(llm.Event{Kind: llm.EvText, Text: " done-" + text})
	emit(llm.Event{Kind: llm.EvDone, Usage: llm.Usage{In: 10 * len(text), Out: len(text)}})
	return nil
}

// start sends a message without waiting for the turn.
func start(a *App, text string) {
	typeRunes(a, text)
	a.onKey(keyK(tcell.KeyEnter))
}

func assistantText(rt *Runtime) string {
	var b strings.Builder
	for _, t := range rt.Session.Snapshot() {
		if t.Role == llm.RoleAssistant {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

func TestTwoSessionsRunConcurrently(t *testing.T) {
	cl := newGateClient()
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	first := a.st.Runtime
	start(a, "alpha")
	a.runCommand("new")
	second := a.st.Runtime
	if first == second || len(a.rtList) != 2 {
		t.Fatal(":new must add a session")
	}
	start(a, "beta")
	if !first.Running || !second.Running {
		t.Fatal("both sessions must run")
	}
	// Both stream; the live text lands in the right session.
	pump(t, a, func() bool { return first.LiveText == "part-alpha" && second.LiveText == "part-beta" })
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "part-beta") || strings.Contains(text, "part-alpha") {
		t.Fatalf("the screen shows the current session only:\n%s", text)
	}
	// Switching mid-turn shows the other stream.
	a.openSession(first.ID())
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "part-alpha") || strings.Contains(text, "part-beta") {
		t.Fatalf("after switching:\n%s", text)
	}
	cl.open("beta")
	pump(t, a, func() bool { return !second.Running })
	if !first.Running || assistantText(second) != "part-beta done-beta" || assistantText(first) != "" {
		t.Fatalf("beta finished first: first running=%v %q / %q", first.Running, assistantText(first), assistantText(second))
	}
	if !second.Unseen || first.Unseen {
		t.Fatal("a turn finished in the background must be flagged unseen")
	}
	if sum, _ := a.sessionsSummary(); sum != "◉ 1 done" {
		t.Fatalf("summary %q", sum)
	}
	cl.open("alpha")
	pump(t, a, func() bool { return !first.Running })
	if assistantText(first) != "part-alpha done-alpha" {
		t.Fatal(assistantText(first))
	}
	// Counters are separate.
	if first.Usage.In != 50 || second.Usage.In != 40 || first.Context != 55 || second.Context != 44 {
		t.Fatalf("usage first %+v ctx %d, second %+v ctx %d", first.Usage, first.Context, second.Usage, second.Context)
	}
	if len(first.Session.Requests) != 1 || len(second.Session.Requests) != 1 {
		t.Fatal("requests recorded in the wrong session")
	}
	a.openSession(second.ID())
	if second.Unseen {
		t.Fatal("viewing clears the flag")
	}
	if sum, _ := a.sessionsSummary(); sum != "" {
		t.Fatalf("summary %q", sum)
	}
}

func TestSessionsDialogShowsLiveStatuses(t *testing.T) {
	cl := newGateClient()
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	first := a.st.Runtime
	start(a, "alpha")
	a.runCommand("new")
	a.runCommand("sessions")
	a.draw()
	text := screenText(scr)
	if !strings.Contains(text, "alpha") || !strings.Contains(text, "current") {
		t.Fatalf("dialog:\n%s", text)
	}
	if g, _ := first.status().glyph(); g != "●" {
		t.Fatal("running glyph")
	}
	if sum, _ := a.sessionsSummary(); sum != "● 1 running" {
		t.Fatalf("summary %q", sum)
	}
	a.closeOverlay()
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "● 1 running") {
		t.Fatalf("status bar lacks the summary:\n%s", text)
	}
	// Enter opens the running session without stopping it.
	a.runCommand("sessions")
	pump(t, a, func() bool { return first.LiveText != "" })
	a.st.Sel = 0
	rows, idx := a.sessionsFiltered()
	for n, i := range idx {
		if rows[i].ID == first.ID() {
			a.st.Sel = n
		}
	}
	a.sessionsKey(keyK(tcell.KeyEnter))
	if a.st.Runtime != first || !first.Running || a.st.Over != OverNone {
		t.Fatal("must switch to the running session")
	}
	cl.open("alpha")
	pump(t, a, func() bool { return !first.Running })
}

func TestBackgroundQuestionWaitsAndIsAnswerableAfterSwitching(t *testing.T) {
	cl := &scriptedRec{}
	cl.script = [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "askUser({questions: [{id: 'q', text: 'which one?', type: 'single', options: [{id: 'x', label: 'x'}, {id: 'y', label: 'y'}]}]}).answers[0].selected[0]"}},
		{{Kind: llm.EvText, Text: "thanks"}, {Kind: llm.EvDone}},
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.st.Settings.Notify = "question"
	a.notifier = func(string, []string) {}
	asker := a.st.Runtime
	start(a, "ask me")
	a.runCommand("new")
	other := a.st.Runtime
	pump(t, a, func() bool { return asker.Ask != nil })
	if other.Ask != nil || a.st.Ask != nil {
		t.Fatal("the question belongs to the first session")
	}
	if g, _ := asker.status().glyph(); g != "?" || asker.status() != statusAsk {
		t.Fatal("waiting glyph")
	}
	if sum, _ := a.sessionsSummary(); sum != "? 1 waiting" {
		t.Fatalf("summary %q", sum)
	}
	a.draw()
	if strings.Contains(screenText(scr), "agent asks") {
		t.Fatal("the panel belongs to the other session")
	}
	// Quitting is refused and names the question.
	a.runCommand("q")
	if a.quit || !strings.Contains(a.lastNotice(), "waiting for an answer") {
		t.Fatalf("q: %q", a.lastNotice())
	}
	a.openSession(asker.ID())
	a.draw()
	if !strings.Contains(screenText(scr), "which one?") {
		t.Fatalf("panel after switching:\n%s", screenText(scr))
	}
	a.onKey(tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone)) // the second option
	a.st.Ask.Focus = focusSubmit
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool { return !asker.Running })
	reqs := cl.requests()
	if len(reqs) != 2 {
		t.Fatalf("%d requests", len(reqs))
	}
	var out string
	for _, m := range reqs[1].Msgs {
		if m.Role == llm.RoleTool {
			out = m.Text
		}
	}
	if out != "y" {
		t.Fatalf("answer not delivered: %q", out)
	}
}
