package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestCtrlCQuitsFromTheLockedFirstRunScreen(t *testing.T) {
	a, _ := newTestApp(t)
	a.st.Locked = true
	a.onKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))
	if !a.quit {
		t.Fatal("ctrl+c must quit while the app is locked on first run")
	}
}

func TestCtrlCDoesNotQuitNormally(t *testing.T) {
	a, _ := newTestApp(t)
	a.onKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))
	if a.quit {
		t.Fatal("ordinary ctrl+c only stops the current request")
	}
}

func TestQuitCommandDetaches(t *testing.T) {
	for _, cmd := range []string{"q", "quit"} {
		a, _ := newTestApp(t)
		a.runCommand(cmd)
		if !a.quit {
			t.Errorf("/%s must leave the terminal", cmd)
		}
	}
}

// The removed commands are ordinary unknown commands and never leave.
func TestRemovedCommandsDoNotLeave(t *testing.T) {
	for _, cmd := range []string{"q!", "quit!", "stop-all"} {
		a, _ := newTestApp(t)
		a.runCommand(cmd)
		if a.quit {
			t.Errorf("/%s must not leave", cmd)
		}
	}
}
