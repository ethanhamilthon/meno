package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
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

func logIndex(a *App, text string) int {
	for i, l := range a.logText() {
		if strings.Contains(string(l), text) {
			return i
		}
	}
	return -1
}

func TestVimCursorWalksIntoTimelineAndBack(t *testing.T) {
	a := visualApp(t)
	top := len(a.logText())
	vimKeys(a, "k")
	if !a.st.InLog || a.st.LogLine != top-1 {
		t.Fatalf("k from the first input row: inLog=%v line=%d top=%d", a.st.InLog, a.st.LogLine, top)
	}
	vimKeys(a, "k")
	if got := string(a.logText()[a.st.LogLine]); !strings.Contains(got, "second line") {
		t.Fatalf("cursor on %q", got)
	}
	vimKeys(a, "gg")
	if a.st.LogLine != 0 {
		t.Fatalf("gg in timeline: %d", a.st.LogLine)
	}
	vimKeys(a, "G")
	if a.st.LogLine != top-1 {
		t.Fatalf("G in timeline: %d", a.st.LogLine)
	}
	vimKeys(a, "j")
	if a.st.InLog {
		t.Fatal("j from the last timeline row must return to the input")
	}
	// editing keys jump back to the input
	vimKeys(a, "kk")
	vimKeys(a, "Ax<esc>")
	if a.st.InLog || string(a.st.Input) != "alpha betax" {
		t.Fatalf("A from the timeline: inLog=%v input=%q", a.st.InLog, string(a.st.Input))
	}
}

func TestVimTimelineIsReadOnlyButYankable(t *testing.T) {
	a := visualApp(t)
	vimKeys(a, "kk")
	for _, k := range []string{"dd", "x", "cw", "J"} {
		a.st.Notices = nil
		vimKeys(a, k)
		if !strings.Contains(a.lastNotice(), "read-only") {
			t.Fatalf("%s must be refused in the timeline", k)
		}
	}
	vimKeys(a, "0fsyw")
	if a.st.Reg != "second " {
		t.Fatalf("yw from the timeline: %q", a.st.Reg)
	}
	vimKeys(a, "yy")
	if !a.st.RegLine || !strings.Contains(a.st.Reg, "second line") {
		t.Fatalf("yy: %q line=%v", a.st.Reg, a.st.RegLine)
	}
	if string(a.st.Input) != "alpha beta" {
		t.Fatal("input changed")
	}
}

func TestVisualSelectsAcrossTimelineAndInput(t *testing.T) {
	a := visualApp(t)
	vimKeys(a, "0vkky")
	// from the input row up through the timeline: anchor at "a" of alpha
	want := a.st.Reg
	if !strings.Contains(want, "second line") || !strings.Contains(want, "a") || a.st.Mode != ui.ModeNormal {
		t.Fatalf("yank across regions: %q mode %v", want, a.st.Mode)
	}
	if string(a.st.Input) != "alpha beta" {
		t.Fatal("yank must not change the input")
	}
	// yank a word out of the timeline and paste it into the input
	a = visualApp(t)
	vimKeys(a, "kk0wviwy")
	if a.st.Reg != "second" {
		t.Fatalf("viw in the timeline: %q", a.st.Reg)
	}
	vimKeys(a, "jjp")
	if !strings.Contains(string(a.st.Input), "second") {
		t.Fatalf("p after yanking from the timeline: %q", string(a.st.Input))
	}
}

func TestVisualEditsInput(t *testing.T) {
	cases := []struct {
		name, keys, want, reg string
	}{
		{"vd", "0vld", "pha beta", "al"},
		{"viwd", "0viwd", " beta", "alpha"},
		{"vawd", "0vawd", "beta", "alpha "},
		{"vc", "0vecX<esc>", "X beta", "alpha"},
		{"$ selects to end", "0wv$d", "alpha ", "beta"},
		{"o swaps", "0wvlohd", "alphata", " be"},
		{"V d", "A<cr>x<esc>ggVd", "x", "alpha beta"},
		{"tilde", "0vl~", "ALpha beta", ""},
		{"U", "0veU", "ALPHA beta", ""},
		{"r", "0vlrz", "zzpha beta", ""},
		{"p replaces", "0veywvep", "alpha alpha", "beta"},
		{"x", "0vlx", "pha beta", "al"},
		{"J", "A<cr>b<esc>ggVjJ", "alpha beta b", ""},
		{"block d", "A<cr>12345<esc>ggl<c-v>jld", "aha beta\n145", "lp\n23"},
		{"v switches to V", "0vVd", "", "alpha beta"},
		{"v again exits", "0vv", "alpha beta", ""},
	}
	for _, c := range cases {
		a := visualApp(t)
		a.st.Session.Replace(nil) // empty timeline: only the input is addressable
		a.setInput(nil)
		a.beginInsert(a.snapshot())
		vimKeys(a, "alpha beta<esc>")
		a.st.Reg = ""
		vimKeys(a, c.keys)
		if got := string(a.st.Input); got != c.want {
			t.Errorf("%s: input %q want %q", c.name, got, c.want)
		}
		if c.reg != "" && a.st.Reg != c.reg {
			t.Errorf("%s: register %q want %q", c.name, a.st.Reg, c.reg)
		}
		if a.st.Mode != ui.ModeNormal && a.st.Mode != ui.ModeInsert {
			t.Errorf("%s: mode %v", c.name, a.st.Mode)
		}
	}
}

func TestVisualDeleteInTimelineIsRefused(t *testing.T) {
	a := visualApp(t)
	vimKeys(a, "kkvld")
	if !strings.Contains(a.lastNotice(), "read-only") || a.st.Mode != ui.ModeVisual {
		t.Fatalf("notice %q mode %v", a.lastNotice(), a.st.Mode)
	}
	vimKeys(a, "<esc>")
	if string(a.st.Input) != "alpha beta" || a.st.Mode != ui.ModeNormal {
		t.Fatal("state after esc")
	}
}

func TestVisualHighlightAndCursorRendering(t *testing.T) {
	a := visualApp(t)
	scr := a.scr.(tcell.SimulationScreen)
	rowOf := func(text string) (int, int) {
		for y := 0; y < 30; y++ {
			if x := strings.Index(visualRow(scr, y), text); x >= 0 {
				return x, y
			}
		}
		t.Fatalf("%q not on screen", text)
		return 0, 0
	}
	bgAt := func(x, y int) tcell.Color {
		_, _, st, _ := scr.GetContent(x, y)
		_, bg, _ := st.Decompose()
		return bg
	}
	// cursor inside the timeline
	vimKeys(a, "kk0")
	a.draw()
	cx, cy, vis := scr.GetCursor()
	x, y := rowOf("second line")
	if !vis || cy != y || cx != ui.PadX {
		t.Fatalf("timeline cursor at %d,%d vis=%v want x=%d y=%d", cx, cy, vis, ui.PadX, y)
	}
	// selection in the timeline
	vimKeys(a, "fsvl")
	a.draw()
	if bgAt(x-1, y) == ui.BgSelect || bgAt(x, y) != ui.BgSelect || bgAt(x+1, y) != ui.BgSelect || bgAt(x+2, y) == ui.BgSelect {
		t.Fatalf("timeline selection bg: %v %v %v", bgAt(x, y), bgAt(x+1, y), bgAt(x+2, y))
	}
	vimKeys(a, "<esc>")
	// linewise selection covers the full row
	vimKeys(a, "V")
	a.draw()
	if bgAt(0, y) != ui.BgSelect || bgAt(60, y) != ui.BgSelect {
		t.Fatal("V must highlight the whole row")
	}
	vimKeys(a, "<esc>jj")
	// selection in the input
	vimKeys(a, "0vl")
	a.draw()
	ix, iy := rowOf("alpha beta")
	if bgAt(ix, iy) != ui.BgSelect || bgAt(ix+1, iy) != ui.BgSelect || bgAt(ix+2, iy) == ui.BgSelect {
		t.Fatalf("input selection bg: %v %v %v", bgAt(ix, iy), bgAt(ix+1, iy), bgAt(ix+2, iy))
	}
	if cx, cy, vis := scr.GetCursor(); !vis || cx != ix+1 || cy != iy {
		t.Fatalf("input cursor %d,%d vis=%v", cx, cy, vis)
	}
}

func TestTimelineCursorScrollsIntoView(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	for i := 0; i < 80; i++ {
		a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "row " + itoa(i)})
	}
	a.setInput(nil)
	a.draw()
	vimKeys(a, "<esc>gg")
	// gg from the input goes to the first input row; walk up into the timeline
	vimKeys(a, "kgg")
	a.draw()
	if a.st.LogLine != 0 || !strings.Contains(screenText(s), "row 0") {
		t.Fatalf("gg must scroll to the top: line %d scroll %d", a.st.LogLine, a.st.Scroll)
	}
	vimKeys(a, "G")
	a.draw()
	if a.st.Scroll != 0 || !strings.Contains(screenText(s), "row 79") {
		t.Fatalf("G must scroll to the bottom: scroll %d", a.st.Scroll)
	}
	vimKeys(a, "20k")
	a.draw()
	if _, cy, vis := s.GetCursor(); !vis || cy < 0 || cy >= 30-a.bottomHeight() {
		t.Fatalf("cursor off the log area: %d", cy)
	}
}
