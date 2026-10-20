package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/proto"
	"meno/internal/ui"
)

func visualRow(s tcell.SimulationScreen, y int) string {
	w, _ := s.Size()
	var b strings.Builder
	for x := 0; x < w; x++ {
		r, _, _, _ := s.GetContent(x, y)
		b.WriteRune(r)
	}
	return b.String()
}

func TestVisualReferenceBottom(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.draw()
	if a.bottomHeight() != 4 {
		t.Fatalf("bottom = %d, want 3 composer + 1 status", a.bottomHeight())
	}
	for y := 32; y < 35; y++ {
		for x := 0; x < 112; x++ {
			_, _, style, _ := s.GetContent(x, y)
			_, bg, _ := style.Decompose()
			if bg != ui.BgTerm {
				t.Fatalf("composer cell %d,%d background %v", x, y, bg)
			}
		}
	}
	_, _, style, _ := s.GetContent(2, 35)
	fg, bg, attrs := style.Decompose()
	if fg != ui.FgBlack || bg != ui.Green || attrs&tcell.AttrReverse != 0 {
		t.Fatalf("badge fg=%v bg=%v attrs=%v", fg, bg, attrs)
	}
	a.globalTasks = 1
	a.globalSessions = []proto.SessionItem{{Running: true}, {Running: true, NeedsYou: true}, {Completed: 2}}
	a.draw()
	row := visualRow(s, 35)
	for _, want := range []string{"running", "tasks", "Asking", "done"} {
		if !strings.Contains(row, want) {
			t.Fatalf("counter missing %s", want)
		}
	}
	exportVisual(t, s, "main")
}

func TestNormalModeComposerKeepsTerminalBackground(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.st.Mode = ui.ModeInput
	a.draw()
	for y := 32; y < 35; y++ {
		for x := 0; x < 112; x++ {
			_, _, style, _ := s.GetContent(x, y)
			_, bg, _ := style.Decompose()
			if bg != ui.BgTerm {
				t.Fatalf("normal-mode composer cell %d,%d background %v", x, y, bg)
			}
		}
	}
}
