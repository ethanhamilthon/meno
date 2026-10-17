package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
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
	code []string // js code to call in the first round of the n-th message, "" = none
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
	a.handle(compactDone{before: 10, after: 5})
	say(t, a, "after compact")
	r = cl.requests()[2]
	if !strings.Contains(r.System, "newer rules") || strings.Contains(lastUser(r), "system-event") {
		t.Fatalf("after compaction: %q", lastUser(r))
	}
}

func TestJSVMResetEventOnlyWithHistory(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.runCommand("reset") // empty history: nothing to tell
	say(t, a, "one")
	if got := lastUser(cl.requests()[0]); got != "one" {
		t.Fatalf("got %q", got)
	}
	a.runCommand("reset")
	say(t, a, "two")
	got := lastUser(cl.requests()[1])
	if !strings.Contains(got, "<js-vm-reset>The JS VM was restarted") || !strings.HasSuffix(got, "two") {
		t.Fatalf("got %q", got)
	}
	say(t, a, "three")
	if got := lastUser(cl.requests()[2]); got != "three" {
		t.Fatalf("the event is delivered once: %q", got)
	}
}

func TestOpeningStoredSessionResetsBaselineAndReportsVM(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	id := a.st.Session.ID
	a.runCommand("new")
	os.WriteFile(filepath.Join(paths.Cwd, "AGENTS.md"), []byte("rules"), 0o644)
	a.resumeSession(id)
	say(t, a, "back")
	r := cl.requests()[len(cl.requests())-1]
	got := lastUser(r)
	if !strings.Contains(r.System, "rules") || strings.Contains(got, "<agents-md-updated>") || !strings.Contains(got, "<js-vm-reset>") {
		t.Fatalf("system rules=%v user=%q", strings.Contains(r.System, "rules"), got)
	}
}

func TestTurnSystemRoundTripsThroughTheDatabase(t *testing.T) {
	dir := t.TempDir()
	s := agent.NewSession("/w", "m")
	s.Append(agent.Turn{Role: llm.RoleUser, Text: "hello", System: "<system-event>x</system-event>"})
	s.Append(agent.Turn{Role: llm.RoleAssistant, Text: "hi"})
	if err := agent.SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	back, err := agent.LoadSession(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Snapshot()[0].System != "<system-event>x</system-event>" || back.Title != "hello" {
		t.Fatalf("%+v title=%q", back.Snapshot()[0], back.Title)
	}
	msgs := back.ToMsgs()
	if msgs[0].Text != "<system-event>x</system-event>\n\nhello" {
		t.Fatalf("msgs[0] = %q", msgs[0].Text)
	}
	agent.CloseSessionStores()
}

func TestPreparationDoesNotBlockTheUI(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	release := make(chan struct{})
	a.prepareHook = func() { <-release }
	typeRunes(a, "slow start")
	start := time.Now()
	a.onKey(keyK(tcell.KeyEnter))
	if d := time.Since(start); d > time.Second {
		t.Fatalf("submit blocked the UI for %s", d)
	}
	if !a.st.Running || a.st.Phase != "preparing" {
		t.Fatalf("running=%v phase=%q", a.st.Running, a.st.Phase)
	}
	// The event loop keeps handling keys and drawing meanwhile.
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "preparing...") {
		t.Fatalf("screen while preparing:\n%s", text)
	}
	typeRunes(a, "next")
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "next") {
		t.Fatalf("typing while preparing:\n%s", text)
	}
	close(release)
	pump(t, a, func() bool { return !a.st.Running })
	if len(cl.requests()) != 1 {
		t.Fatal("request not sent")
	}
}

func TestDiffBaselineKinds(t *testing.T) {
	mk := func() *baseline {
		return &baseline{
			Cwd:    "/a",
			Skills: map[string]skillSig{"B": {"/p/B.md", "h1"}, "D": {"/p/D.md", "h"}},
			Agents: map[string]fileSig{"global": {}, "project": {Path: "/a/AGENTS.md", Hash: "x", Exists: true}},
			MCP:    map[string]mcpSig{"gh": {State: "running", Tools: []string{"mcp.gh.old()"}}},
			Date:   "2026-10-17",
		}
	}
	cur := mk()
	cur.Skills["B"] = skillSig{"/p/B.md", "h2"}
	cur.Skills["C"] = skillSig{"/p/C.md", "h"}
	delete(cur.Skills, "D")
	cur.Agents["project"] = fileSig{Path: "/a/AGENTS.md", Hash: "y", Exists: true}
	cur.MCP["gh"] = mcpSig{State: "running", Tools: []string{"mcp.gh.search({q: string})"}}
	cur.MCP["db"] = mcpSig{State: "failed", Err: "boom"}
	cur.Date = "2026-10-18"
	cur.Cwd = "/b"
	got := strings.Join(diffBaseline(mk(), cur), "\n")
	for _, want := range []string{
		"<skills-updated>Skill \"B\" was updated: reread /p/B.md before you use it.</skills-updated>",
		"<skills-added>Skill \"C\" is now available: /p/C.md</skills-added>",
		"<skills-removed>Skill \"D\" is no longer available.</skills-removed>",
		"<agents-md-updated>Project instructions changed: reread /a/AGENTS.md.</agents-md-updated>",
		"<mcp-updated>Server \"gh\" changed its tools. New tools: mcp.gh.search({q: string}). Removed tools: mcp.gh.old().</mcp-updated>",
		"<mcp-updated>Server \"db\" failed: boom</mcp-updated>",
		"<date-changed>The date is now 2026-10-18, Sunday.</date-changed>",
		"<cwd-changed>The working directory changed from /a to /b. Relative paths and sh() now use /b.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if len(diffBaseline(mk(), mk())) != 0 || len(diffBaseline(nil, cur)) != 0 {
		t.Fatal("equal baselines give no events")
	}
	// Server added, then removed.
	gone := mk()
	delete(gone.MCP, "gh")
	if got := strings.Join(diffBaseline(mk(), gone), "\n"); !strings.Contains(got, "Server \"gh\" is no longer running") {
		t.Fatal(got)
	}
	conn := mk()
	conn.MCP["new"] = mcpSig{State: "running", Tools: []string{"mcp.new.x()"}}
	if got := strings.Join(diffBaseline(mk(), conn), "\n"); !strings.Contains(got, "Server \"new\" connected. New tools: mcp.new.x().") {
		t.Fatal(got)
	}
	if b := eventBlock(nil); b != "" {
		t.Fatal("no lines, no block")
	}
}

func TestCwdChangeKeepsPromptAndReportsEverything(t *testing.T) {
	// :cd arrives in stage 3; the mechanism is a diff between two worlds.
	old := &baseline{Cwd: "/a", Skills: map[string]skillSig{}, Agents: map[string]fileSig{"project": {Path: "/a/AGENTS.md", Hash: "x", Exists: true}}, MCP: map[string]mcpSig{}}
	cur := &baseline{Cwd: "/b", Skills: map[string]skillSig{"s": {"/b/.meno/skills/s.md", "h"}}, Agents: map[string]fileSig{"project": {Path: "/b/AGENTS.md", Hash: "y", Exists: true}}, MCP: map[string]mcpSig{}}
	got := strings.Join(diffBaseline(old, cur), "\n")
	for _, want := range []string{"<cwd-changed>", "<skills-added>Skill \"s\"", "reread /b/AGENTS.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}
