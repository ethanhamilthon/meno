package app

import (
	"github.com/gdamore/tcell/v2"
	"meno/internal/agent"
	"meno/internal/llm"
	"testing"
)

type repaintScreen struct {
	tcell.SimulationScreen
	shows, syncs int
}

func (s *repaintScreen) Show() { s.shows++; s.SimulationScreen.Show() }
func (s *repaintScreen) Sync() { s.syncs++; s.SimulationScreen.Sync() }

func TestUnchangedFrameDoesNotRepaint(t *testing.T) {
	a, screen := visualFixture(t, 80, 20)
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "existing conversation"})
	a.setInput([]rune("stable draft"))
	counted := &repaintScreen{SimulationScreen: screen}
	a.scr = counted
	a.draw()
	first := counted.shows
	a.draw()
	a.draw()
	if counted.shows != first || counted.syncs != 0 {
		t.Fatalf("unchanged frame repainted: shows=%d first=%d sync=%d", counted.shows, first, counted.syncs)
	}
	a.setInput([]rune("changed draft"))
	a.draw()
	if counted.shows != first+1 {
		t.Fatal("changed draft did not repaint")
	}
}
