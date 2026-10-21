package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/proto"
	"meno/internal/ui"
)

// statusRow draws only the status bar at width w and returns its text and the
// foreground color of every cell.
func statusRow(t *testing.T, a *App, scr tcell.SimulationScreen, w int) (string, []tcell.Color) {
	t.Helper()
	scr.SetSize(w, 10)
	scr.Clear()
	a.drawStatus(ui.Canvas{S: scr}, 0, 2, w)
	var b strings.Builder
	fgs := make([]tcell.Color, w)
	for x := 0; x < w; x++ {
		r, _, st, _ := scr.GetContent(x, 2)
		b.WriteRune(r)
		fgs[x], _, _ = st.Decompose()
	}
	return b.String(), fgs
}

func countersApp(t *testing.T) (*App, tcell.SimulationScreen) {
	a, scr := newTestApp(t)
	a.globalTasks = 2
	a.globalSessions = []proto.SessionItem{{Running: true}, {Running: true, NeedsYou: true}, {Completed: 2, ReadThrough: 1}}
	return a, scr
}

// Each counter keeps its own color on the bar, and the " | " separators are dim.
func TestStatusCountersKeepTheirColors(t *testing.T) {
	a, scr := countersApp(t)
	defer scr.Fini()
	row, fgs := statusRow(t, a, scr, 120)
	for text, want := range map[string]tcell.Color{"1 running": ui.Green, "2 tasks": ui.Purple, "1 Asking": ui.Orange, "1 done": ui.Blue} {
		i := strings.Index(row, text)
		if i < 0 {
			t.Fatalf("%q missing in %q", text, row)
		}
		if fgs[i] != want || fgs[i+len(text)-1] != want {
			t.Errorf("%q colored %v, want %v", text, fgs[i], want)
		}
	}
	if i := strings.Index(row, " | "); i < 0 || fgs[i+1] != ui.FgDim {
		t.Errorf("separator color %v", fgs[strings.Index(row, " | ")+1])
	}
}

// When the bar is narrow the counters are shortened but stay colored, and
// they never overlap the left or right part.
func TestStatusCountersCompactAndStayColored(t *testing.T) {
	a, scr := countersApp(t)
	defer scr.Fini()
	for _, w := range []int{80, 64, 48} {
		row, fgs := statusRow(t, a, scr, w)
		if i := strings.Index(row, "1r"); i >= 0 {
			if fgs[i] != ui.Green {
				t.Errorf("w=%d compact counter color %v", w, fgs[i])
			}
		}
		if strings.Count(row, "model")+strings.Count(row, "/") == 0 && w >= 64 {
			t.Errorf("w=%d right part lost: %q", w, row)
		}
	}
}

// A broken stream replaces the counters and is drawn in the warning color.
func TestStatusOfflineIsColored(t *testing.T) {
	a, scr := countersApp(t)
	defer scr.Fini()
	a.connLost = map[string]bool{a.st.Runtime.ID(): true}
	row, fgs := statusRow(t, a, scr, 120)
	i := strings.Index(row, "offline")
	if i < 0 || fgs[i] != ui.Orange {
		t.Fatalf("offline %q color %v", row, fgs)
	}
}
