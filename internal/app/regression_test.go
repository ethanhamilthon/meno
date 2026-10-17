package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

func TestTimelineScrollFixedBottom(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: strings.Repeat("timeline row\n\n", 100)})
	a.st.Input = []rune("fixed input")
	a.draw()
	_, h := s.Size()
	before := screenText(s)
	bottom := strings.Split(before, "\n")[h-4 : h]
	a.onKey(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	a.draw()
	if a.st.Scroll == 0 {
		t.Fatal("page up did not scroll")
	}
	after := strings.Split(screenText(s), "\n")[h-4 : h]
	if strings.Join(bottom, "\n") != strings.Join(after, "\n") {
		t.Fatal("composer/status moved")
	}
	a.scrollBy(1 << 25)
	maxScroll := a.st.Scroll
	a.scrollBy(1)
	if a.st.Scroll != maxScroll {
		t.Fatal("scroll not clamped")
	}
	a.handle(tcell.NewEventMouse(1, 1, tcell.WheelDown, tcell.ModNone))
	if a.st.Scroll >= maxScroll {
		t.Fatal("mouse wheel ignored")
	}
	a.st.Mode = ui.ModeNormal // in insert mode End moves the text cursor
	a.onKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if a.st.Scroll != 0 {
		t.Fatal("end did not follow tail")
	}
}
func TestCommandCompletion(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Mode = ui.ModeCommand
	a.st.Cmd = []rune("pro")
	a.onCommandKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	a.onCommandKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if string(a.st.Cmd) != "providers" {
		t.Fatalf("completion: %s", string(a.st.Cmd))
	}
	a.st.Cmd = []rune("set")
	a.st.CmdSel = 0
	a.onCommandKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if a.st.Over != OverSettings {
		t.Fatal("enter did not accept completion")
	}
	a.closeOverlay()
	a.st.Mode = ui.ModeCommand
	a.st.Cmd = []rune("export output.md")
	a.onCommandKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if string(a.st.Cmd) != "export output.md" {
		t.Fatal("arguments overwritten")
	}
}
func TestSmallScreenAndLongUnicode(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "# Hello\n\n**你好** 👩🏽‍💻 é\n\n```go\nlong line\n```"})
	for _, size := range [][2]int{{1, 1}, {4, 2}, {15, 5}, {40, 10}} {
		s.SetSize(size[0], size[1])
		for _, m := range []ui.Mode{ui.ModeInsert, ui.ModeNormal, ui.ModeCommand} {
			a.st.Mode = m
			a.draw()
		}
	}
}
func TestMissingProviderKeepsInput(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Input = []rune("keep this")
	a.submit()
	if string(a.st.Input) != "keep this" {
		t.Fatal("lost input")
	}
}
func TestRunningSessionGuards(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	id := a.st.Session.ID
	a.st.Running = true
	for _, cmd := range []string{"reset", "compact", "cd /"} {
		a.st.Notices = nil
		a.runCommand(cmd)
		if a.st.Session.ID != id || a.quit || !strings.Contains(a.lastNotice(), ":stop") {
			t.Fatalf("unsafe command %s", cmd)
		}
	}
	a.st.Notices = nil
	a.runCommand("q")
	if a.quit || !strings.Contains(a.lastNotice(), "not quitting") || !strings.Contains(a.lastNotice(), "running") || !strings.Contains(a.lastNotice(), ":q!") {
		t.Fatalf("q while running: quit=%v notice %q", a.quit, a.lastNotice())
	}
	for cmd, over := range map[string]Overlay{"settings": OverSettings, "model": OverModel, "mcp": OverMCP, "skills": OverSkills, "providers": OverProviders, "help": OverHelp, "sessions": OverSessions} {
		a.closeOverlay()
		a.runCommand(cmd)
		if a.st.Over != over {
			t.Fatalf(":%s must open while the agent runs", cmd)
		}
	}
	// :new does not touch the running session.
	a.closeOverlay()
	a.runCommand("new")
	if a.st.Session.ID == id || a.st.Running || !a.rts[id].Running {
		t.Fatal(":new must leave the running session alone and show a fresh one")
	}
}
func TestCloseReleasesFullEventQueue(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	for len(a.events) < cap(a.events) {
		a.events <- nil
	}
	finished := make(chan struct{})
	go func() { a.postEvent(turnDone{}); close(finished) }()
	a.Close()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("shutdown deadlocked")
	}
}
func TestUsageAccumulatesAndPersists(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Providers = []llm.Provider{{Name: "openai"}}
	a.st.Settings.Provider = "openai"
	a.st.Settings.Model = "test"
	a.st.Pricing = map[string]llm.ModelInfo{"openai/test": {Input: 2, Output: 10, Context: 10000}}
	for i := 0; i < 2; i++ {
		a.onAgent(agent.Ev{Kind: agent.EvUsage, Usage: llm.Usage{In: 1000, Out: 100, Cached: 500}})
	}
	if a.st.Usage.In != 2000 || a.st.Context != 1100 || a.st.Spent < 0.0059 {
		t.Fatalf("wrong stats %+v", a.st)
	}
	a.onTurnDone(nil)
	id := a.st.Session.ID
	a.runCommand("new")
	a.dropRuntime(a.rts[id]) // reopen from the database
	a.openSession(id)
	if a.st.Usage.In != 2000 || a.st.Context != 1100 {
		t.Fatal("resume lost stats")
	}
}
