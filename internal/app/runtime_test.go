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
		{{Kind: llm.EvTool, CallID: "c1", Code: "askUser('which one?', {choices: ['x', 'y']})"}},
		{{Kind: llm.EvText, Text: "thanks"}, {Kind: llm.EvDone}},
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.st.Settings.NotifyQuestion = "on"
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
	a.onKey(tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone)) // the second choice
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

func TestQuitAndForceQuit(t *testing.T) {
	cl := newGateClient()
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	start(a, "alpha")
	a.runCommand("new")
	a.runCommand("q")
	if a.quit || !strings.Contains(a.lastNotice(), "alpha") || !strings.Contains(a.lastNotice(), "running") {
		t.Fatalf("q must refuse and name the session: %q", a.lastNotice())
	}
	a.runCommand("q!")
	if !a.quit {
		t.Fatal("q! quits")
	}
	pump(t, a, func() bool { return !a.rtList[0].Running })
	// A session that is idle does not hold q back.
	b, scr2, _ := e2eApp(t, newGateClient())
	defer scr2.Fini()
	b.runCommand("q")
	if !b.quit {
		t.Fatal("idle q")
	}
}

func TestCdChangesTheSessionDirectory(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	sub := filepath.Join(paths.Cwd, "sub")
	os.MkdirAll(filepath.Join(sub, ".meno", "skills"), 0o755)
	os.WriteFile(filepath.Join(sub, ".meno", "skills", "local.md"), []byte("---\ndescription: local\n---\n"), 0o644)
	os.WriteFile(filepath.Join(sub, "marker.txt"), []byte("in sub"), 0o644)
	os.WriteFile(filepath.Join(paths.Cwd, "marker.txt"), []byte("in root"), 0o644)
	other := filepath.Join(a.paths.Home, "..")
	_ = other

	// Validation.
	a.runCommand("cd nonexistent")
	if !strings.Contains(a.lastNotice(), "no such directory") || a.st.Cwd != paths.Cwd {
		t.Fatalf("missing dir: %q", a.lastNotice())
	}
	a.runCommand("cd marker.txt")
	if !strings.Contains(a.lastNotice(), "not a directory") {
		t.Fatalf("file: %q", a.lastNotice())
	}
	a.runCommand("cd -")
	if !strings.Contains(a.lastNotice(), "no previous directory") {
		t.Fatalf("dash: %q", a.lastNotice())
	}

	// Empty history: nothing else, the prompt is built for the new directory.
	a.runCommand("cd sub")
	if a.st.Cwd != sub || a.st.VM.Root() != sub || a.st.Session.Cwd != sub {
		t.Fatalf("cwd %q root %q session %q", a.st.Cwd, a.st.VM.Root(), a.st.Session.Cwd)
	}
	say(t, a, "first")
	r := cl.requests()[0]
	if lastUser(r) != "first" || !strings.Contains(r.System, "Working directory: "+sub) || !strings.Contains(r.System, "- local") {
		t.Fatalf("first request: user %q\n%s", lastUser(r), r.System)
	}

	// Relative paths: fs and sh follow, variables survive.
	if res := a.st.VM.Exec(context.Background(), "let kept = 7; fs.read('marker.txt')"); res.Value != "in sub" {
		t.Fatalf("%+v", res)
	}
	a.runCommand("cd ..")
	if a.st.Cwd != paths.Cwd || a.st.PrevCwd != sub {
		t.Fatalf("cd ..: %q prev %q", a.st.Cwd, a.st.PrevCwd)
	}
	if res := a.st.VM.Exec(context.Background(), "fs.read('marker.txt') + kept"); res.Value != "in root7" {
		t.Fatalf("%+v", res)
	}
	// cd - goes back; non-empty history: the prompt stays, events tell the model.
	a.runCommand("cd -")
	a.runCommand("cd ~")
	home, _ := os.UserHomeDir()
	if a.st.Cwd != home {
		t.Fatalf("~: %q", a.st.Cwd)
	}
	a.runCommand("cd " + paths.Cwd)
	say(t, a, "second")
	r2 := cl.requests()[1]
	got := lastUser(r2)
	if r2.System != r.System {
		t.Fatal("cd must not rebuild the prompt")
	}
	if !strings.Contains(got, "<cwd-changed>The working directory changed from "+sub+" to "+paths.Cwd) || !strings.Contains(got, "<skills-removed>Skill \"local\"") {
		t.Fatalf("event: %q", got)
	}
	// The directory is saved with the session.
	saved, err := agent.LoadSession(a.paths.Home, a.st.Session.ID)
	if err != nil || saved.Cwd != paths.Cwd {
		t.Fatalf("saved cwd %q err %v", saved.Cwd, err)
	}
}

func TestCdEventOnlyWithHistory(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "one")
	sub := filepath.Join(paths.Cwd, "sub")
	os.MkdirAll(filepath.Join(sub, ".meno", "skills"), 0o755)
	os.WriteFile(filepath.Join(sub, ".meno", "skills", "local.md"), []byte("---\ndescription: local\n---\n"), 0o644)
	os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("sub rules"), 0o644)
	a.runCommand("cd sub")
	say(t, a, "two")
	r := cl.requests()[1]
	got := lastUser(r)
	for _, want := range []string{
		"<cwd-changed>The working directory changed from " + paths.Cwd + " to " + sub,
		"<skills-added>Skill \"local\"",
		"<agents-md-updated>Project instructions were added: read " + filepath.Join(sub, "AGENTS.md"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(r.System, "sub rules") || !strings.Contains(r.System, "Working directory: "+paths.Cwd) {
		t.Fatal("the frozen prompt must not change")
	}
}

func TestCdRefusedWhileRunning(t *testing.T) {
	cl := newGateClient()
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	os.Mkdir(filepath.Join(paths.Cwd, "sub"), 0o755)
	start(a, "alpha")
	a.runCommand("cd sub")
	if a.st.Cwd != paths.Cwd || !strings.Contains(a.lastNotice(), ":stop") {
		t.Fatalf("cd while running: %q", a.lastNotice())
	}
	cl.open("alpha")
	pump(t, a, func() bool { return !a.st.Running })
}

func TestNewSessionStartsInTheCurrentDirectory(t *testing.T) {
	a, scr, paths := e2eApp(t, &recClient{})
	defer scr.Fini()
	sub := filepath.Join(paths.Cwd, "sub")
	os.Mkdir(sub, 0o755)
	a.runCommand("cd sub")
	a.runCommand("new")
	if a.st.Cwd != sub || a.st.VM.Root() != sub || a.st.Session.Cwd != sub {
		t.Fatalf("new session cwd %q", a.st.Cwd)
	}
	// Each session has its own directory.
	other := a.rts[a.rtList[0].ID()]
	if other != nil && other.ID() != a.st.Session.ID && other.VM == a.st.VM {
		t.Fatal("sessions share a VM")
	}
}

func TestCdCompletion(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"alpha", "alps", "beta", ".hidden"} {
		os.Mkdir(filepath.Join(dir, d), 0o755)
	}
	os.WriteFile(filepath.Join(dir, "alfile"), nil, 0o644)
	got := completeDirs(dir, "al")
	if len(got) != 2 || got[0] != ":cd alpha/" || got[1] != ":cd alps/" {
		t.Fatalf("%v", got)
	}
	if got := completeDirs(dir, ""); len(got) != 3 {
		t.Fatalf("hidden directories need a dot: %v", got)
	}
	if got := completeDirs("/", dir+"/b"); len(got) != 1 || got[0] != ":cd "+dir+"/beta/" {
		t.Fatalf("absolute: %v", got)
	}
	if got := completeDirs(filepath.Dir(dir), "./"+filepath.Base(dir)+"/be"); len(got) != 1 {
		t.Fatalf("relative: %v", got)
	}
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Cwd = dir
	a.st.Cmd = []rune("cd al")
	rows, ok := a.cdRows()
	if !ok || len(rows) != 2 {
		t.Fatalf("%v %v", rows, ok)
	}
	a.st.Mode = 3 // command mode
	a.onCommandKey(keyK(tcell.KeyTab))
	if string(a.st.Cmd) != "cd alpha/" {
		t.Fatalf("tab: %q", string(a.st.Cmd))
	}
}

func TestOpeningStoredSessionWithMissingDirectory(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	gone := filepath.Join(paths.Cwd, "gone")
	os.Mkdir(gone, 0o755)
	a.runCommand("cd gone")
	say(t, a, "one")
	id := a.st.Session.ID
	a.runCommand("new")
	a.dropRuntime(a.rts[id])
	os.Remove(gone)

	a.openSession(id)
	if a.st.Session.ID != id || a.st.Cwd != paths.Cwd || a.st.VM.Root() != paths.Cwd {
		t.Fatalf("must open in the launch directory, got %q", a.st.Cwd)
	}
	if n := a.lastNotice(); !strings.Contains(n, "no longer exists") || !strings.Contains(n, gone) {
		t.Fatalf("notice %q", n)
	}
	say(t, a, "again")
	r := cl.requests()[1]
	got := lastUser(r)
	if !strings.Contains(got, "<cwd-changed>The working directory changed from "+gone+" to "+paths.Cwd) || !strings.Contains(got, "<js-vm-reset>") {
		t.Fatalf("events: %q", got)
	}
	if !strings.Contains(r.System, "Working directory: "+paths.Cwd) {
		t.Fatal("prompt must be built for the launch directory")
	}
}

func TestSessionRowsMergeMemoryAndDatabase(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "stored one")
	id := a.st.Session.ID
	a.runCommand("new")
	a.dbRows, _ = agent.ListRecentSessions(a.paths.Home, 200)
	rows := a.sessionRows()
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	for _, r := range rows {
		if r.ID == id && (r.rt == nil || r.Title != "stored one" || r.Turns != 2) {
			t.Fatalf("row %+v", r)
		}
	}
	// Dropping the runtime leaves the database row.
	a.dropRuntime(a.rts[id])
	rows = a.sessionRows()
	found := false
	for _, r := range rows {
		if r.ID == id {
			found = r.rt == nil && r.Title == "stored one"
		}
	}
	if !found {
		t.Fatalf("rows %+v", rows)
	}
}

func TestEmptyNewSessionIsNotKeptWhenLeft(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	say(t, a, "work")
	a.runCommand("new")
	a.runCommand("new")
	a.runCommand("new")
	if len(a.rtList) != 2 {
		t.Fatalf("%d runtimes: empty unused sessions pile up", len(a.rtList))
	}
	a.runCommand("quit")
	a.Close()
	list, _ := agent.ListSessions(a.paths.Home)
	if len(list) != 1 {
		t.Fatalf("empty sessions must not be stored, got %d", len(list))
	}
}

var _ = time.Second
