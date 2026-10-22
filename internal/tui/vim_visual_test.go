package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/render"
)

// visualApp has a short timeline ("hello world", "second line" as user turns)
// and the input "alpha beta" in normal mode.
func visualApp(t *testing.T) *App {
	t.Helper()
	a, s := newTestApp(t)
	t.Cleanup(s.Fini)
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "hello world"})
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "second line"})
	a.setInput(nil)
	vimKeys(a, "alpha beta<esc>")
	return a
}

func TestVisualDraftAndHistoryEntry(t *testing.T) {
	a := visualApp(t)
	a.st.Cursor = 0
	a.runCommand("edit")
	vimKeys(a, "v")
	if a.st.Mode != render.ModeVisual || a.st.InLog {
		t.Fatal("draft not selected")
	}
	vimKeys(a, "kgg")
	if !a.st.InLog || a.st.LogLine != 0 {
		t.Fatal("history motion failed")
	}
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Mode != render.ModeEdit {
		t.Fatal("exit not EDIT")
	}
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Mode != render.ModeInput {
		t.Fatal("exit not INPUT")
	}
	a.setInput(nil)
	a.runCommand("edit")
	vimKeys(a, "v")
	if !a.st.InLog || a.st.Mode != render.ModeVisual {
		t.Fatal("empty draft must select history")
	}
}

func TestVisualEditsInput(t *testing.T) {
	for _, tc := range []struct{ keys, want string }{{"ld", "pha beta"}, {"iwU", "ALPHA beta"}, {"ly", "alpha beta"}} {
		a := visualApp(t)
		a.st.Session.Replace(nil)
		a.setInput([]rune("alpha beta"))
		a.st.Cursor = 0
		a.runCommand("edit")
		vimKeys(a, "v")
		vimKeys(a, tc.keys)
		if string(a.st.Input) != tc.want {
			t.Fatalf("%s: %q", tc.keys, string(a.st.Input))
		}
		if a.st.Mode != render.ModeInput {
			t.Fatalf("mode %v", a.st.Mode)
		}
	}
}

func TestVisualDeleteInTimelineIsRefused(t *testing.T) {
	a := visualApp(t)
	a.setInput(nil)
	a.runCommand("edit")
	vimKeys(a, "v")
	vimKeys(a, "ld")
	if !strings.Contains(a.lastNotice(), "read-only") || a.st.Mode != render.ModeVisual {
		t.Fatalf("%s %v", a.lastNotice(), a.st.Mode)
	}
	a.onKey(keyK(tcell.KeyEsc))
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Mode != render.ModeInput {
		t.Fatal("exit failed")
	}
}

func TestVisualHighlightAndCursorRendering(t *testing.T) {
	a := visualApp(t)
	a.st.Cursor = 0
	a.runCommand("edit")
	vimKeys(a, "v")
	vimKeys(a, "l")
	a.draw()
	scr := a.scr.(tcell.SimulationScreen)
	x, y, visible := scr.GetCursor()
	if !visible || x < 0 || y < 0 {
		t.Fatal("cursor missing")
	}
	_, _, style, _ := scr.GetContent(x, y)
	_, bg, _ := style.Decompose()
	if bg != render.BgSelect {
		t.Fatal("selection not highlighted")
	}
}

func TestTimelineCursorScrollsIntoView(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	for i := 0; i < 80; i++ {
		a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "row " + itoa(i)})
	}
	a.setInput(nil)
	a.runCommand("edit")
	vimKeys(a, "v")
	vimKeys(a, "gg")
	a.draw()
	if a.st.LogLine != 0 || !strings.Contains(screenText(s), "row 0") {
		t.Fatal("top not visible")
	}
	vimKeys(a, "G")
	a.draw()
	if !strings.Contains(screenText(s), "row 79") {
		t.Fatal("bottom not visible")
	}
}
