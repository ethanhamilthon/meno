package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

// vimKeys feeds keys; "<esc>" is Escape, "<c-r>" is Ctrl+R.
func vimKeys(a *App, keys string) {
	rs := []rune(keys)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '<' {
			if j := indexRune(rs[i:], '>'); j > 0 {
				switch string(rs[i : i+j+1]) {
				case "<esc>":
					a.onKey(tcell.NewEventKey(tcell.KeyEsc, 0, 0))
				case "<c-r>":
					a.onKey(tcell.NewEventKey(tcell.KeyCtrlR, 0, 0))
				case "<c-v>":
					a.onKey(tcell.NewEventKey(tcell.KeyCtrlV, 0, 0))
				case "<cr>":
					a.onKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModShift))
				}
				i += j
				continue
			}
		}
		a.onKey(tcell.NewEventKey(tcell.KeyRune, rs[i], 0))
	}
}

func indexRune(rs []rune, r rune) int {
	for i, x := range rs {
		if x == r {
			return i
		}
	}
	return -1
}

func TestInputCommandDrafts(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	vimKeys(a, "hello :hjkl")
	if string(a.st.Input) != "hello :hjkl" {
		t.Fatal(string(a.st.Input))
	}
	a.onKey(keyK(tcell.KeyEsc))
	vimKeys(a, "settings")
	if a.st.Mode != ui.ModeCommand || string(a.st.Cmd) != "settings" {
		t.Fatalf("%v %q", a.st.Mode, string(a.st.Cmd))
	}
	a.onKey(keyK(tcell.KeyEsc))
	if string(a.st.Input) != "hello :hjkl" {
		t.Fatal("message lost")
	}
	a.onKey(keyK(tcell.KeyEsc))
	if string(a.st.Cmd) != "settings" {
		t.Fatal("command lost")
	}
}
func TestColonIsOrdinaryInput(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	vimKeys(a, ":model")
	if a.st.Mode != ui.ModeInput || string(a.st.Input) != ":model" {
		t.Fatal("colon intercepted")
	}
}
