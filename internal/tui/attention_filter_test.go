package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/api"
)

func filterApp(t *testing.T) (*App, tcell.SimulationScreen) {
	a, scr := newTestApp(t)
	a.globalSessions = []api.SessionItem{
		{ID: "run", Name: "running one", Running: true},
		{ID: "ask", Name: "asking one", Running: true, NeedsYou: true},
		{ID: "done", Name: "done one", Completed: 2, ReadThrough: 1},
		{ID: "idle", Name: "idle one"},
	}
	return a, scr
}

func titles(a *App) []string {
	rows, idx := a.sessionsFiltered()
	var out []string
	for _, i := range idx {
		out = append(out, rows[i].Title)
	}
	return out
}

// Tab walks through the attention filters; each shows only its sessions and
// asking wins over running, like the counters of the status bar.
func TestSessionsFilterByAttention(t *testing.T) {
	a, scr := filterApp(t)
	defer scr.Fini()
	a.openOverlay(OverSessions)
	if got := titles(a); len(got) != 4 {
		t.Fatalf("all: %v", got)
	}
	want := map[int]string{filterRunning: "running one", filterAsking: "asking one", filterDone: "done one"}
	for f := filterRunning; f <= filterDone; f++ {
		a.onKey(keyK(tcell.KeyTab))
		if got := titles(a); len(got) != 1 || got[0] != want[f] {
			t.Errorf("filter %s: %v", filterNames[f], got)
		}
	}
	a.draw()
	if !strings.Contains(screenText(scr), "filter: done") {
		t.Fatal("the active filter is not shown")
	}
	a.onKey(keyK(tcell.KeyTab))
	if a.st.AttnFilter != filterAll || len(titles(a)) != 4 {
		t.Fatal("tab did not wrap to all")
	}
}

// A click on a counter opens /sessions with that counter's filter.
func TestClickOnCounterOpensFilteredSessions(t *testing.T) {
	a, scr := filterApp(t)
	defer scr.Fini()
	scr.SetSize(120, 30)
	a.draw()
	_, h := scr.Size()
	row := visualRow(scr, h-1)
	i := strings.Index(row, "1 Asking")
	if i < 0 {
		t.Fatalf("no Asking counter in %q", row)
	}
	a.onMouse(tcell.NewEventMouse(i+1, h-1, tcell.Button1, 0))
	if a.st.Over != OverSessions || a.st.AttnFilter != filterAsking {
		t.Fatalf("over=%v filter=%d", a.st.Over, a.st.AttnFilter)
	}
}
