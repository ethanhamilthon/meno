package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

// typeKeys feeds keys; "<esc>" is Escape, "<cr>" is Shift+Enter (a newline).
func typeKeys(a *App, keys string) {
	rs := []rune(keys)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '<' {
			if j := indexRune(rs[i:], '>'); j > 0 {
				switch string(rs[i : i+j+1]) {
				case "<esc>":
					a.onKey(tcell.NewEventKey(tcell.KeyEsc, 0, 0))
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

func TestColonIsOrdinaryInput(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	typeKeys(a, "a:model")
	if a.st.Mode != render.ModeInput || string(a.st.Input) != "a:model" {
		t.Fatal("colon inside a word intercepted")
	}
}
