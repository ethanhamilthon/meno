package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// testQuery sets the search text of a list dialog opened by hand in a test.
func testQuery(a *App, q string) {
	if a.st.Entry == nil {
		a.beginEntry(entry{}, "")
	}
	a.setQuery(q)
}

func TestEntryPutsTheDraftAsideAndBringsItBack(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.setInput([]rune("my draft"))
	a.st.Cursor = 3
	a.st.Undo = []snap{{in: []rune("old")}}
	a.beginEntry(entry{label: "x "}, "typed")
	if string(a.st.Input) != "typed" || len(a.st.Undo) != 0 {
		t.Fatalf("the entry must start clean: %q undo=%d", string(a.st.Input), len(a.st.Undo))
	}
	typeRunes(a, "!")
	a.endEntry()
	if string(a.st.Input) != "my draft" || a.st.Cursor != 3 || len(a.st.Undo) != 1 {
		t.Fatalf("draft not restored: %q cursor=%d undo=%d", string(a.st.Input), a.st.Cursor, len(a.st.Undo))
	}
}

func TestModalEntryEnterAndEsc(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	var got string
	fail := true
	a.setInput([]rune("draft"))
	a.openOverlay(OverSettings)
	a.beginEntry(entry{done: func(s string) error {
		if fail {
			return errors.New("not yet")
		}
		got = s
		return nil
	}}, "")
	typeRunes(a, "abc")
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Entry == nil || got != "" {
		t.Fatal("an error must keep the entry")
	}
	fail = false
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Entry != nil || got != "abc" || string(a.st.Input) != "draft" {
		t.Fatalf("enter must end the entry: got=%q input=%q", got, string(a.st.Input))
	}
	cancelled := false
	a.beginEntry(entry{cancel: func() { cancelled = true }, done: func(string) error { t.Fatal("esc must not confirm"); return nil }}, "x")
	a.onKey(keyK(tcell.KeyEsc))
	if !cancelled || a.st.Entry != nil || string(a.st.Input) != "draft" {
		t.Fatal("esc must cancel and restore")
	}
}

func TestEntryIsPlainText(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.openOverlay(OverSettings)
	a.beginEntry(entry{done: func(string) error { return nil }}, "")
	typeRunes(a, "/model @x")
	if a.entryText() != "/model @x" || a.st.Over != OverSettings {
		t.Fatalf("slash and @ are text in an entry: %q", a.entryText())
	}
	a.entryPaste("one\ntwo  three\n")
	if a.entryText() != "/model @xone two three" {
		t.Fatalf("a pasted newline must become a space: %q", a.entryText())
	}
	a.onKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt))
	typeRunes(a, "|")
	a.onKey(keyK(tcell.KeyCtrlU))
	if a.entryText() != "" {
		t.Fatalf("ctrl+u must clear: %q", a.entryText())
	}
}

func TestEntryEndsWhenAnotherSessionComesOnScreen(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.setInput([]rune("first"))
	first := a.st.Runtime
	a.beginEntry(entry{done: func(string) error { return nil }}, "x")
	a.st.Runtime = a.newRuntime(first.Session, first.Cwd)
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Entry != nil || string(first.Input) != "first" {
		t.Fatalf("the draft of the first session must be back: %q", string(first.Input))
	}
}

func TestEntryDrawsLabelAndBullets(t *testing.T) {
	a, s := visualFixture(t, 90, 30)
	a.openOverlay(OverSettings)
	a.beginEntry(entry{label: "api key ", secret: true, done: func(string) error { return nil }}, "hunter2")
	a.draw()
	text := screenText(s)
	if !strings.Contains(text, "api key •••••••") || strings.Contains(text, "hunter2") {
		t.Fatalf("label and bullets expected:\n%s", text)
	}
}
