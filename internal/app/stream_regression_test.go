package app

import (
	"context"
	"github.com/gdamore/tcell/v2"
	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
	"strings"
	"testing"
)

func TestLiveStreamVisibleBeforeFinalTurn(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Running = true
	a.onAgent(agent.Ev{Kind: agent.EvText, Text: "**live** answer"})
	a.draw()
	text := screenText(s)
	if !strings.Contains(text, "live answer") || strings.Contains(text, "**") {
		t.Fatalf("stream not rendered: %s", text)
	}
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "**live** answer"})
	a.onAgent(agent.Ev{Kind: agent.EvDone})
	a.draw()
	if strings.Count(screenText(s), "live answer") != 1 {
		t.Fatal("final answer duplicated")
	}
}
func TestCtrlCCancelsTurn(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.st.cancel = cancel
	a.onKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))
	if ctx.Err() == nil {
		t.Fatal("ctrl-c did not cancel turn")
	}
}
func TestScrolledViewportAnchoredWhileNewRowsArrive(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: strings.Repeat("row\n\n", 100)})
	a.draw()
	a.scrollBy(10)
	a.draw()
	old := a.st.Scroll
	oldLines := a.st.LogLines
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "new row"})
	a.draw()
	if a.st.Scroll != old+a.st.LogLines-oldLines {
		t.Fatalf("viewport moved: old %d new %d", old, a.st.Scroll)
	}
}

func TestSubmittedMessageVisibleSynchronously(t *testing.T) {
	a, s, _ := e2eApp(t, &scriptedClient{})
	defer s.Fini()
	defer a.Close()
	a.st.Input = []rune("show this immediately")
	a.submit()
	turns := a.st.Session.Snapshot()
	if len(turns) == 0 || turns[0].Role != llm.RoleUser || turns[0].Text != "show this immediately" {
		t.Fatalf("user message missing before events: %+v", turns)
	}
	a.draw()
	if !strings.Contains(screenText(s), "show this immediately") || strings.Contains(screenText(s), "write a task") && len(a.st.Input) != 0 {
		t.Fatal("submitted message not rendered")
	}
	pump(t, a, func() bool { return !a.st.Running })
	count := 0
	for _, turn := range a.st.Session.Snapshot() {
		if turn.Role == llm.RoleUser {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("user message duplicated: %d", count)
	}
}

func TestCommandModeIsOneUnpaddedLineAtTheBottom(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Mode = ui.ModeCommand
	a.st.Cmd = []rune("hello") // matches no command: only the input line remains
	a.draw()
	_, h := s.Size()
	if a.bottomHeight() != 1 {
		t.Fatalf("bottom = %d, want a single line", a.bottomHeight())
	}
	line := visualRow(s, h-1)
	if !strings.HasPrefix(line, ":hello") {
		t.Fatalf("command line %q", line)
	}
	for _, label := range []string{"COMMAND", "INSERT", "NORMAL", "model", "↓"} {
		for y := 0; y < h; y++ {
			if y >= h-1 && strings.Contains(visualRow(s, y), label) {
				t.Fatalf("status content %q leaked into the command line: %q", label, visualRow(s, y))
			}
		}
	}
	_, _, st, _ := s.GetContent(0, h-1)
	if _, bg, _ := st.Decompose(); bg != ui.BgUser {
		t.Fatal("command line background")
	}
	// With matching commands the completion list sits directly above, left-aligned.
	a.st.Cmd = []rune("mo")
	a.draw()
	if a.bottomHeight() < 2 {
		t.Fatal("completion rows missing")
	}
	top := visualRow(s, h-a.bottomHeight())
	if strings.TrimSpace(top) == "" || !strings.HasPrefix(top, "  ▸ :") && !strings.HasPrefix(top, "    :") {
		t.Fatalf("first completion row not left-aligned: %q", top)
	}
}

func cursorVisible(s tcell.SimulationScreen) bool {
	_, _, vis := s.GetCursor()
	return vis
}
