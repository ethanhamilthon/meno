package tui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func headerFG(s tcell.SimulationScreen, y, w int) []tcell.Color {
	cells, _, _ := s.GetContents()
	out := make([]tcell.Color, w)
	for x := 0; x < w; x++ {
		fg, _, _ := cells[y*w+x].Style.Decompose()
		out[x] = fg
	}
	return out
}

func TestComposerWaveKeepsTextAndFooterStyles(t *testing.T) {
	a, s := visualFixture(t, 80, 20)
	a.setInput([]rune("readable text"))
	a.draw()
	y := a.st.ComposerY
	text := visualRow(s, y)
	foreground := headerFG(s, y, 80)
	footer := headerFG(s, 19, 80)
	base := time.Now()
	a.waveEpoch = base
	a.now = func() time.Time { return base.Add(1500 * time.Millisecond) }
	a.st.Running = true
	a.draw()
	if visualRow(s, y) != text {
		t.Fatal("wave changed text")
	}
	for x, c := range foreground {
		if headerFG(s, y, 80)[x] != c {
			t.Fatal("wave changed foreground")
		}
	}
	for x, c := range footer {
		if headerFG(s, 19, 80)[x] != c {
			t.Fatal("wave changed footer")
		}
	}
	for _, row := range []int{y - 1, y + 1} {
		colors := composerBackgrounds(s, row, 80)
		for x, c := range composerBackgrounds(s, y, 80) {
			if colors[x] != c {
				t.Fatal("padding wave phase differs")
			}
		}
	}
}
