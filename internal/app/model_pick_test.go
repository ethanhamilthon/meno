package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Arrow keys in the picker only preview an effort; the session keeps its own
// until Enter, and Esc throws the candidate away.
func TestEffortPickIsLocalUntilEnter(t *testing.T) {
	a, _ := newTestApp(t)
	a.openOverlay(OverModel)
	before := effortOf(a)
	a.modelKey(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	if a.st.EffortPick == "" || a.st.EffortPick == before && len(a.effortsFor(a.highlightedPair())) > 1 {
		t.Fatalf("candidate not set: %q (current %q)", a.st.EffortPick, before)
	}
	if effortOf(a) != before {
		t.Fatalf("session effort changed to %q before Enter", effortOf(a))
	}
	a.onOverlayKey(tcell.NewEventKey(tcell.KeyEsc, 0, 0))
	if effortOf(a) != before || a.st.EffortPick != "" {
		t.Fatalf("Esc must cancel: effort=%q pick=%q", effortOf(a), a.st.EffortPick)
	}
}
