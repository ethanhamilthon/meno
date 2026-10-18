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

func TestVimEditsInput(t *testing.T) {
	cases := []struct {
		name, keys, want string
	}{
		{"cw", "hello world<esc>0cwbye<esc>", "bye world"},
		{"dw", "one two three<esc>0wdw", "one three"},
		{"dd+p", "a<cr>b<cr>c<esc>ggddp", "b\na\nc"},
		{"yyp", "a<cr>b<esc>ggyyp", "a\na\nb"},
		{"x count", "abcdef<esc>03x", "def"},
		{"D", "abc def<esc>0wD", "abc "},
		{"ciw", "foo bar<esc>0wciwX<esc>", "foo X"},
		{"ci quote", "say \"hi there\" now<esc>0f\"ci\"Y<esc>", "say \"Y\" now"},
		{"o", "a<esc>ob<esc>", "a\nb"},
		{"O", "b<esc>Oa<esc>", "a\nb"},
		{"A/I", "mid<esc>Aend<esc>Istart<esc>", "startmidend"},
		{"r", "cat<esc>0rb", "bat"},
		{"~", "ab<esc>0~~", "AB"},
		{"J", "a<cr>b<esc>ggJ", "a b"},
		{"dt", "foo(bar)<esc>0dt(", "(bar)"},
		{"2dw", "a b c d<esc>02dw", "c d"},
		{"undo", "abc<esc>xxu", "ab"},
		{"undo insert", "abc<esc>Adef<esc>u", "abc"},
		{"redo", "abc<esc>xu<c-r>", "ab"},
		{"cc", "old line<esc>ccnew<esc>", "new"},
		{"dG", "a<cr>b<cr>c<esc>ggjdG", "a"},
		{"p count", "ab<esc>0yl3p", "aaaab"},
		{"semicolon", "a.b.c.d<esc>0f.;x", "a.bc.d"},
	}
	for _, c := range cases {
		a, s := newTestApp(t)
		a.onKey(tcell.NewEventKey(tcell.KeyRune, 'x', 0)) // typed in insert mode
		a.setInput(nil)
		a.st.Undo, a.st.InsBase = nil, a.snapshot()
		vimKeys(a, c.keys)
		if got := string(a.st.Input); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
		s.Fini()
	}
}

func TestSpaceOpensCommandLineInNormalMode(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	typeRunes(a, "abc")
	a.onKey(keyK(tcell.KeyEsc))
	a.onKey(key(' '))
	if a.st.Mode != ui.ModeCommand || a.st.Cmd != nil {
		t.Fatalf("space in normal mode: mode %v cmd %q", a.st.Mode, string(a.st.Cmd))
	}
	a.onKey(keyK(tcell.KeyEsc))
	// With a count pending, space still moves right (like l).
	a.onKey(key('0'))
	a.onKey(key('2'))
	a.onKey(key(' '))
	if a.st.Mode != ui.ModeNormal || a.st.Cursor != 2 {
		t.Fatalf("2<space>: mode %v cursor %d", a.st.Mode, a.st.Cursor)
	}
}

func TestVimModeTransitions(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	vimKeys(a, "ab<esc>")
	if a.st.Mode != ui.ModeNormal || a.st.Cursor != 1 {
		t.Fatalf("esc must step back: mode %v cursor %d", a.st.Mode, a.st.Cursor)
	}
	vimKeys(a, "i")
	if a.st.Mode != ui.ModeInsert {
		t.Fatal("i must enter insert mode")
	}
}
