package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/ui"
)

// styleScreen records the requested cursor style (SimulationScreen does not expose it).
type styleScreen struct {
	tcell.SimulationScreen
	style tcell.CursorStyle
}

func (s *styleScreen) SetCursorStyle(cs tcell.CursorStyle, c ...tcell.Color) {
	s.style = cs
	s.SimulationScreen.SetCursorStyle(cs, c...)
}

func typeText(a *App, s string) {
	for _, r := range s {
		a.onKey(tcell.NewEventKey(tcell.KeyRune, r, 0))
	}
}

func press(a *App, key tcell.Key, mod tcell.ModMask) { a.onKey(tcell.NewEventKey(key, 0, mod)) }

func TestComposerNewlineKeys(t *testing.T) {
	for name, ev := range map[string]*tcell.EventKey{
		"shift-enter": tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModShift),
		"alt-enter":   tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModAlt),
		"ctrl-j":      tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl),
	} {
		t.Run(name, func(t *testing.T) {
			a, _ := visualFixture(t, 60, 20)
			a.setInput([]rune("one"))
			a.onKey(ev)
			typeText(a, "t")
			if got := string(a.st.Input); got != "one\nt" {
				t.Fatalf("input = %q", got)
			}
			if a.st.Running {
				t.Fatal("newline must not submit")
			}
		})
	}
}

func TestComposerGrowsAndShowsAllLines(t *testing.T) {
	a, s := visualFixture(t, 60, 20)
	base := a.bottomHeight()
	a.setInput([]rune("alpha\nbeta\ngamma"))
	if got := a.bottomHeight(); got != base+2 {
		t.Fatalf("bottom = %d, want %d", got, base+2)
	}
	a.draw()
	var screen []string
	for y := 0; y < 20; y++ {
		screen = append(screen, visualRow(s, y))
	}
	all := strings.Join(screen, "\n")
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(all, want) {
			t.Fatalf("%q missing in:\n%s", want, all)
		}
	}
}

func TestComposerCapsAndWraps(t *testing.T) {
	a, _ := visualFixture(t, 40, 30)
	a.setInput([]rune(strings.Repeat("x\n", 50)))
	if got := a.composerTextRows(40); got != composerMaxText {
		t.Fatalf("rows = %d", got)
	}
	// Width 10 keeps one column free for the cursor, so rows hold 9 cells.
	rows := layoutInput([]rune(strings.Repeat("a", 25)), 10)
	if len(rows) != 3 || rows[0] != (vrow{0, 9, true}) || rows[2] != (vrow{18, 25, false}) {
		t.Fatalf("wrap = %+v", rows)
	}
	if got := layoutInput([]rune("a\n"), 10); len(got) != 2 || got[1] != (vrow{2, 2, false}) {
		t.Fatalf("trailing newline = %+v", got)
	}
}

func TestInsertCursorMovementAndEditing(t *testing.T) {
	a, _ := visualFixture(t, 60, 20)
	typeText(a, "héllo wörld")
	press(a, tcell.KeyLeft, 0)
	press(a, tcell.KeyLeft, 0)
	typeText(a, "X")
	if got := string(a.st.Input); got != "héllo wörXld" {
		t.Fatalf("insert in middle: %q", got)
	}
	press(a, tcell.KeyBackspace2, 0)
	press(a, tcell.KeyDelete, 0)
	if got := string(a.st.Input); got != "héllo wörd" {
		t.Fatalf("backspace+delete: %q", got)
	}
	press(a, tcell.KeyHome, 0)
	typeText(a, ">")
	press(a, tcell.KeyEnd, 0)
	typeText(a, "<")
	if got := string(a.st.Input); got != ">héllo wörd<" {
		t.Fatalf("home/end: %q", got)
	}
	press(a, tcell.KeyLeft, tcell.ModAlt)
	typeText(a, "|")
	if got := string(a.st.Input); got != ">héllo |wörd<" {
		t.Fatalf("word left: %q", got)
	}
	press(a, tcell.KeyRight, tcell.ModAlt)
	typeText(a, "|")
	if got := string(a.st.Input); got != ">héllo |wörd<|" {
		t.Fatalf("word right: %q", got)
	}
	// Arrows stop at the ends instead of wrapping around.
	for i := 0; i < 50; i++ {
		press(a, tcell.KeyLeft, 0)
	}
	if a.st.Cursor != 0 {
		t.Fatalf("cursor = %d", a.st.Cursor)
	}
	press(a, tcell.KeyBackspace2, 0)
	if got := string(a.st.Input); got != ">héllo |wörd<|" {
		t.Fatalf("backspace at start: %q", got)
	}
}

func TestInsertUpDownMovesBetweenRows(t *testing.T) {
	a, _ := visualFixture(t, 60, 20)
	a.setInput([]rune("first line\nab\nthird line"))
	press(a, tcell.KeyUp, 0) // col 10 on "third line" -> clamps to end of "ab"
	if a.st.Cursor != len("first line\nab") {
		t.Fatalf("up 1: cursor %d", a.st.Cursor)
	}
	press(a, tcell.KeyUp, 0) // sticky column: back to col 10 on first line
	if a.st.Cursor != len("first line") {
		t.Fatalf("up 2: cursor %d", a.st.Cursor)
	}
	press(a, tcell.KeyUp, 0) // top row: jump to start
	if a.st.Cursor != 0 {
		t.Fatalf("up 3: cursor %d", a.st.Cursor)
	}
	press(a, tcell.KeyDown, 0)
	press(a, tcell.KeyDown, 0)
	press(a, tcell.KeyDown, 0) // bottom row: jump to end
	if a.st.Cursor != len(a.st.Input) {
		t.Fatalf("down: cursor %d", a.st.Cursor)
	}
	// Wrapped rows count as rows too.
	a.setInput([]rune(strings.Repeat("a", 100)))
	a.st.Cursor = 5
	press(a, tcell.KeyDown, 0)
	if a.st.Cursor <= 5 || a.st.Cursor >= 100 {
		t.Fatalf("down across wrap: cursor %d", a.st.Cursor)
	}
}

func TestColonIsPlainTextInInsertMode(t *testing.T) {
	a, _ := visualFixture(t, 60, 20)
	typeText(a, "/model")
	if a.st.Mode != ui.ModeInput || string(a.st.Input) != "/model" {
		t.Fatalf("mode=%v input=%q", a.st.Mode, string(a.st.Input))
	}
}

func TestCursorBlinksOnlyInInsertMode(t *testing.T) {
	a, sim := visualFixture(t, 60, 20)
	s := &styleScreen{SimulationScreen: sim}
	a.scr = s
	a.setInput([]rune("hi"))
	a.draw()
	x, y, vis := s.GetCursor()
	wantY := a.logHeight() + 1 // the first text row follows the header
	if !vis || x != composerPadX+2 || y != wantY {
		t.Fatalf("insert cursor %d,%d vis=%v", x, y, vis)
	}
	if s.style != tcell.CursorStyleBlinkingBar {
		t.Fatalf("insert style %v", s.style)
	}
	a.st.Mode = ui.ModeVisual
	a.draw()
	if _, _, vis := s.GetCursor(); !vis || s.style != tcell.CursorStyleSteadyBlock {
		t.Fatalf("normal: vis=%v style=%v", vis, s.style)
	}
	a.st.Mode = ui.ModeVisual
	a.draw()
	if _, _, vis := s.GetCursor(); !vis || s.style != tcell.CursorStyleSteadyBlock {
		t.Fatalf("visual: vis=%v style=%v", vis, s.style)
	}
	a.st.Mode = ui.ModeInput
	a.openOverlay(OverHelp)
	a.draw()
	if _, _, vis := s.GetCursor(); vis {
		t.Fatal("cursor must be hidden under a dialog")
	}
}

func TestPlaceholderHasThreeTexts(t *testing.T) {
	a, s := visualFixture(t, 90, 20)
	row := func() string { a.draw(); return screenText(s) }
	if got := row(); !strings.Contains(got, "Type to start building anything") {
		t.Fatalf("new session: %q", got)
	}
	a.st.Running = true
	for _, phase := range []string{"thinking", "running", "writing", "x"} {
		a.st.Phase = phase
		a.st.RunStart = time.Now().Add(-5 * time.Second)
		got := row()
		if !strings.Contains(got, "Tell the working agent something") || strings.Contains(got, phase+"...") || strings.Contains(got, "0:05") {
			t.Fatalf("phase %q: %q", phase, got)
		}
	}
	a.st.Running = false
	a.st.LiveText = "an answer"
	a.st.Phase = ""
	if got := row(); !strings.Contains(got, "Type your next task here") {
		t.Fatalf("after an answer: %q", got)
	}
	a.setInput([]rune("typed"))
	if got := row(); strings.Contains(got, "Type your next task") || !strings.Contains(got, "typed") {
		t.Fatalf("typed: %q", got)
	}
}

func refFixture(t *testing.T) *App {
	a, _ := visualFixture(t, 100, 30)
	a.st.Skills = []config.Skill{{Name: "api-security", Desc: "auth checks", Enabled: true}, {Name: "docker-deploy", Enabled: false}}
	return a
}
