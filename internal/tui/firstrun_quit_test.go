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

func TestQuitCommandAndLegacyBangBothDetach(t *testing.T) {
	for _, cmd := range []string{"q", "q!", "quit", "quit!"} {
		a, _ := newTestApp(t)
		a.runCommand(cmd)
		if !a.quit {
			t.Errorf("/%s must leave the terminal", cmd)
		}
	}
}

func TestStopAllNeedsConfirmation(t *testing.T) {
	a, _ := newTestApp(t)
	a.st.Runtime.Running = true
	a.runCommand("stop-all")
	if !a.stopAllAsk.After(a.stopAllAsk.Add(-1)) || a.stopAllAsk.IsZero() {
		t.Fatal("the first /stop-all only asks")
	}
	if !a.st.Runtime.Running {
		t.Fatal("nothing may be stopped before confirmation")
	}
}
