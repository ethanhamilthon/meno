package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

// selectComposer drags the mouse from column 0 of the first composer row to
// the given display column.
func selectComposer(a *App, to int) {
	a.draw()
	x, y := a.st.ComposerX, a.st.ComposerY
	a.onMouse(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	a.onMouse(tcell.NewEventMouse(x+to, y, tcell.Button1, tcell.ModNone))
	a.onMouse(tcell.NewEventMouse(x+to, y, tcell.ButtonNone, tcell.ModNone))
}

func TestSaveSnippetFromComposerSelection(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	typeRunes(a, "hello world")
	selectComposer(a, 10)
	if a.st.Mode != ui.ModeVisual {
		t.Fatalf("drag did not select: mode %v", a.st.Mode)
	}
	if a.st.SelText != "hello world" {
		t.Fatalf("selection %q", a.st.SelText)
	}
	a.runCommand("save_snippet greet")
	sn, err := a.storeForTest().Snippet("greet")
	if err != nil || sn.Text != "hello world" {
		t.Fatalf("stored %+v %v", sn, err)
	}
}

func TestComposerSelectionFollowsWrappedRows(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	typeRunes(a, strings.Repeat("ab", 50))
	a.draw()
	rows := a.st.ComposerRows
	if len(rows) < 2 {
		t.Fatalf("input did not wrap: %+v", rows)
	}
	row := rows[1]
	x, y := a.st.ComposerX, a.st.ComposerY+1-a.st.ComposerTop
	a.onMouse(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	a.onMouse(tcell.NewEventMouse(x+9, y, tcell.Button1, tcell.ModNone))
	a.onMouse(tcell.NewEventMouse(x+9, y, tcell.ButtonNone, tcell.ModNone))
	want := string(a.st.Input[row.start:min(row.start+10, len(a.st.Input))])
	if a.st.SelText != want {
		t.Fatalf("selection %q, want %q", a.st.SelText, want)
	}
}

func TestSaveSnippetNeedsSelection(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.runCommand("save_snippet empty")
	if _, err := a.storeForTest().Snippet("empty"); err == nil {
		t.Fatal("saved a snippet without a selection")
	}
}

func TestPasteSnippetInsertsIntoComposer(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	if err := a.storeForTest().SaveSnippet("greet", "hello world"); err != nil {
		t.Fatal(err)
	}
	typeRunes(a, "say: ")
	a.onKey(keyK(tcell.KeyEsc))
	a.st.Cursor = len(a.st.Input) // after the space, as if the cursor was at the end
	a.runCommand("paste_snippet greet")
	if got := string(a.st.Input); got != "say: hello world" {
		t.Fatalf("input %q", got)
	}
	if a.st.Mode != ui.ModeInsert || a.st.Cursor != len(a.st.Input) {
		t.Fatalf("mode %v cursor %d", a.st.Mode, a.st.Cursor)
	}
	// Typing continues right after the snippet.
	typeRunes(a, "!")
	if got := string(a.st.Input); got != "say: hello world!" {
		t.Fatalf("after typing %q", got)
	}
}

func TestSnippetsModalRenamesAndDeletes(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	for _, sn := range [][2]string{{"one", "a"}, {"two", "b"}} {
		if err := a.storeForTest().SaveSnippet(sn[0], sn[1]); err != nil {
			t.Fatal(err)
		}
	}
	a.runCommand("snippets")
	if a.st.Over != OverSnippets || len(a.st.Snippets) != 2 {
		t.Fatalf("modal %v with %d snippets", a.st.Over, len(a.st.Snippets))
	}
	a.onKey(keyK(tcell.KeyDown))
	a.onKey(key('r'))
	for range "two" {
		a.onKey(keyK(tcell.KeyBackspace2))
	}
	typeRunes(a, "second")
	a.onKey(keyK(tcell.KeyEnter))
	if _, err := a.storeForTest().Snippet("second"); err != nil {
		t.Fatalf("rename failed: %v", err)
	}
	if a.st.Snippets[a.st.Sel].Name != "second" {
		t.Fatalf("cursor on %q", a.st.Snippets[a.st.Sel].Name)
	}
	a.onKey(key('d'))
	if len(a.st.Snippets) != 2 {
		t.Fatal("one d must only arm the delete")
	}
	a.onKey(key('d'))
	if len(a.st.Snippets) != 1 || a.st.Snippets[0].Name != "one" {
		t.Fatalf("after delete %+v", a.st.Snippets)
	}
	if _, err := a.storeForTest().Snippet("second"); err == nil {
		t.Fatal("snippet still stored")
	}
	a.draw()
	if out := screenText(scr); !containsAll(out, "snippets", "one") {
		t.Fatalf("modal not drawn:\n%s", out)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
