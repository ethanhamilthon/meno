package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
	"meno/internal/prompt"
)

// systemPrompt builds the prompt for the current state, synchronously.
func (a *App) systemPrompt() string {
	w, warnings := a.worldSrc().gather(context.Background())
	for _, m := range warnings {
		a.notifyErr(m)
	}
	return prompt.Build(w.Input)
}

// recClient records every request and answers with a short text.
type recClient struct {
	mu   sync.Mutex
	reqs []llm.Request
}

func (c *recClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	c.mu.Unlock()
	emit(llm.Event{Kind: llm.EvText, Text: "ok"})
	emit(llm.Event{Kind: llm.EvDone})
	return nil
}

func (c *recClient) requests() []llm.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]llm.Request(nil), c.reqs...)
}

// say sends a message and waits for the turn to end.
func say(t *testing.T, a *App, text string) {
	t.Helper()
	typeRunes(a, text)
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool { return !a.st.Running })
}

// lastUser returns the text of the last user message of a request.
func lastUser(r llm.Request) string {
	for i := len(r.Msgs) - 1; i >= 0; i-- {
		if r.Msgs[i].Role == llm.RoleUser {
			return r.Msgs[i].Text
		}
	}
	return ""
}

func writeSkill(t *testing.T, a *App, name, body string) string {
	t.Helper()
	dir := a.paths.ProjectSkillsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name+".md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSkillEditBetweenMessagesProducesOneEvent(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	path := writeSkill(t, a, "b", "---\ndescription: first\n---\nv1\n")
	say(t, a, "one")
	writeSkill(t, a, "b", "---\ndescription: first\n---\nv2\n")
	say(t, a, "two")
	reqs := cl.requests()
	if len(reqs) != 2 {
		t.Fatalf("%d requests", len(reqs))
	}
	if strings.Contains(lastUser(reqs[0]), "<system-event>") {
		t.Fatal("the first message after a reset point carries no event")
	}
	want := "<skills-updated>Skill \"b\" was updated: reread " + path + " before you use it.</skills-updated>"
	got := lastUser(reqs[1])
	if !strings.HasPrefix(got, "<system-event>This is a message from the system itself, not a user instruction.\n"+want+"\n</system-event>\n\ntwo") {
		t.Fatalf("second message:\n%s", got)
	}
	if reqs[0].System != reqs[1].System {
		t.Fatal("the system prompt must stay frozen")
	}
	// Nothing changed: no event.
	say(t, a, "three")
	if got := lastUser(cl.requests()[2]); got != "three" {
		t.Fatalf("third message: %q", got)
	}
	// Stored on the turn, hidden from the title.
	turns := a.st.Session.Snapshot()
	if turns[2].System == "" || turns[2].Text != "two" {
		t.Fatalf("turn: %+v", turns[2])
	}
	a.saveSession()
	if a.st.Session.Title != "one" {
		t.Fatalf("title %q", a.st.Session.Title)
	}
}

func TestFrozenPromptSurvivesChangesAndEventsCombine(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	// Several changes: one block. Added-then-removed cancels out.
	writeSkill(t, a, "new", "---\ndescription: n\n---\n")
	writeSkill(t, a, "temp", "---\ndescription: t\n---\n")
	os.Remove(filepath.Join(a.paths.ProjectSkillsDir(), "temp.md"))
	os.WriteFile(filepath.Join(paths.Cwd, "AGENTS.md"), []byte("rules"), 0o644)
	say(t, a, "two")
	reqs := cl.requests()
	got := lastUser(reqs[1])
	if strings.Count(got, "<system-event>") != 1 || !strings.Contains(got, "<skills-added>Skill \"new\"") ||
		!strings.Contains(got, "<agents-md-updated>Project instructions were added: read "+filepath.Join(paths.Cwd, "AGENTS.md")) ||
		strings.Contains(got, "temp") {
		t.Fatalf("event block:\n%s", got)
	}
	if reqs[0].System != reqs[1].System || strings.Contains(reqs[1].System, "rules") {
		t.Fatal("the system prompt must not change mid-segment")
	}
}

func TestAddThenRemoveProducesNoEvent(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	p := writeSkill(t, a, "temp", "x")
	os.Remove(p)
	say(t, a, "two")
	if got := lastUser(cl.requests()[1]); got != "two" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentsChangeIsReportedToTheAgentItself(t *testing.T) {
	// The agent edits AGENTS.md during a turn; the next message reports it.
	cl := &scriptedRec{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	agentsFile := filepath.Join(paths.Cwd, "AGENTS.md")
	os.WriteFile(agentsFile, []byte("v1"), 0o644)
	cl.script = [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "fs.write(" + quote(agentsFile) + ", 'v2')"}},
		{{Kind: llm.EvText, Text: "done"}, {Kind: llm.EvDone}},
	}
	say(t, a, "edit it")
	if strings.Contains(lastUser(cl.requests()[len(cl.requests())-1]), "system-event") {
		t.Fatal("a change made during the turn belongs to the next message")
	}
	cl.script = nil
	say(t, a, "next")
	got := lastUser(cl.requests()[len(cl.requests())-1])
	if !strings.Contains(got, "<agents-md-updated>Project instructions changed: reread "+agentsFile+".") {
		t.Fatalf("got %q", got)
	}
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "\\'") + "'" }

// scriptedRec replays scripted rounds, then answers "ok"; it records requests.
type scriptedRec struct {
	recClient
	script [][]llm.Event
}

func (c *scriptedRec) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	var evs []llm.Event
	if len(c.script) > 0 {
		evs, c.script = c.script[0], c.script[1:]
	} else {
		evs = []llm.Event{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}
	}
	c.mu.Unlock()
	for _, e := range evs {
		emit(e)
	}
	return nil
}

func TestResetPointsRebuildWithoutEvents(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	os.WriteFile(filepath.Join(paths.Cwd, "AGENTS.md"), []byte("fresh rules"), 0o644)

	// :new is a reset point: the prompt reflects the current state, no event.
	a.runCommand("new")
	say(t, a, "in new")
	r := cl.requests()[1]
	if !strings.Contains(r.System, "fresh rules") || strings.Contains(lastUser(r), "system-event") {
		t.Fatalf("after :new: system has rules=%v, user %q", strings.Contains(r.System, "fresh rules"), lastUser(r))
	}

	// So is a finished compaction.
	os.WriteFile(filepath.Join(paths.Cwd, "AGENTS.md"), []byte("newer rules"), 0o644)
	a.handle(compactDone{sid: a.st.Session.ID, before: 10, after: 5})
	say(t, a, "after compact")
	r = cl.requests()[2]
	if !strings.Contains(r.System, "newer rules") || strings.Contains(lastUser(r), "system-event") {
		t.Fatalf("after compaction: %q", lastUser(r))
	}
}

func TestOpeningStoredSessionResetsBaseline(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	id := a.st.Session.ID
	a.runCommand("new")
	os.WriteFile(filepath.Join(paths.Cwd, "AGENTS.md"), []byte("rules"), 0o644)
	a.dropRuntime(a.rts[id]) // force a load from the database
	a.openSession(id)
	say(t, a, "back")
	r := cl.requests()[len(cl.requests())-1]
	got := lastUser(r)
	if !strings.Contains(r.System, "rules") || strings.Contains(got, "<agents-md-updated>") {
		t.Fatalf("system rules=%v user=%q", strings.Contains(r.System, "rules"), got)
	}
}
