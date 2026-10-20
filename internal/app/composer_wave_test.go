package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
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

func TestHeaderWaveRunsOnlyWhileWorking(t *testing.T) {
	a, s := visualFixture(t, 80, 20)
	y := a.logHeight()
	base := time.Now()
	a.waveEpoch = base
	a.now = func() time.Time { return base.Add(1500 * time.Millisecond) }
	a.draw()
	idle := headerFG(s, y, 80)
	for x, c := range idle {
		if c != ui.FgFaint && c != chromeText() && c != tcell.ColorDefault {
			t.Fatalf("idle header col %d has color %v", x, c)
		}
	}
	a.st.Running = true
	a.draw()
	r1 := headerFG(s, y, 80)
	lit := 0
	for _, c := range r1 {
		if c != ui.FgFaint && c != chromeText() {
			lit++
		}
	}
	if lit == 0 {
		t.Fatal("no wave while running")
	}
	a.now = func() time.Time { return base.Add(2500 * time.Millisecond) }
	a.draw()
	r2 := headerFG(s, y, 80)
	same := true
	for i := range r1 {
		same = same && r1[i] == r2[i]
	}
	if same {
		t.Fatal("wave does not move")
	}
	a.st.Running = false
	a.draw()
	for x, c := range headerFG(s, y, 80) {
		if c != ui.FgFaint && c != chromeText() && c != tcell.ColorDefault {
			t.Fatalf("after run col %d has color %v", x, c)
		}
	}
}

func TestHeaderTextSameWithAndWithoutWave(t *testing.T) {
	a, s := visualFixture(t, 80, 20)
	y := a.logHeight()
	a.draw()
	idle := visualRow(s, y)
	a.st.Running = true
	a.draw()
	if got := visualRow(s, y); got != idle {
		t.Fatalf("header text changed:\n%q\n%q", idle, got)
	}
	if !strings.Contains(idle, "─") {
		t.Fatalf("no rule: %q", idle)
	}
}
