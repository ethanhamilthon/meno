package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func ek(key tcell.Key, r rune, m tcell.ModMask) *tcell.EventKey { return tcell.NewEventKey(key, r, m) }

func editType(t *textEdit, s string) {
	for _, r := range s {
		t.Handle(ek(tcell.KeyRune, r, 0))
	}
}

func TestTextEditInsertsAtCursor(t *testing.T) {
	var e textEdit
	editType(&e, "helo")
	e.Handle(ek(tcell.KeyLeft, 0, 0))
	editType(&e, "l")
	if e.String() != "hello" || e.Cursor != 4 {
		t.Fatalf("got %q cursor %d", e.String(), e.Cursor)
	}
}

func TestTextEditDeleteKeys(t *testing.T) {
	var e textEdit
	e.Set("one two three")
	e.Handle(ek(tcell.KeyCtrlW, 0, 0))
	if e.String() != "one two " {
		t.Fatalf("ctrl+w: %q", e.String())
	}
	e.Handle(ek(tcell.KeyHome, 0, 0))
	e.Handle(ek(tcell.KeyDelete, 0, 0))
	if e.String() != "ne two " {
		t.Fatalf("delete: %q", e.String())
	}
	e.Handle(ek(tcell.KeyCtrlK, 0, 0))
	if e.String() != "" {
		t.Fatalf("ctrl+k: %q", e.String())
	}
	e.Set("abc def")
	e.Handle(ek(tcell.KeyCtrlU, 0, 0))
	if e.String() != "" {
		t.Fatalf("ctrl+u: %q", e.String())
	}
}

func TestTextEditWordMoves(t *testing.T) {
	var e textEdit
	e.Set("alpha beta gamma")
	e.Handle(ek(tcell.KeyLeft, 0, tcell.ModAlt))
	if e.Cursor != 11 {
		t.Fatalf("alt+left: %d", e.Cursor)
	}
	e.Handle(ek(tcell.KeyRight, 0, tcell.ModCtrl))
	if e.Cursor != 16 {
		t.Fatalf("ctrl+right: %d", e.Cursor)
	}
	e.Handle(ek(tcell.KeyBackspace2, 0, tcell.ModAlt))
	if e.String() != "alpha beta " {
		t.Fatalf("alt+backspace: %q", e.String())
	}
}

func TestTextEditGraphemes(t *testing.T) {
	var e textEdit
	e.Set("a👍🏽b")
	e.Handle(ek(tcell.KeyLeft, 0, 0))
	e.Handle(ek(tcell.KeyBackspace2, 0, 0))
	if e.String() != "ab" {
		t.Fatalf("emoji with a skin tone must go as one: %q", e.String())
	}
}

func TestTextEditSingleLineKeys(t *testing.T) {
	var e textEdit
	e.Set("x")
	for _, k := range []*tcell.EventKey{ek(tcell.KeyEnter, 0, 0), ek(tcell.KeyUp, 0, 0), ek(tcell.KeyDown, 0, 0), ek(tcell.KeyEsc, 0, 0), ek(tcell.KeyTab, 0, 0), ek(tcell.KeyRune, 'q', tcell.ModCtrl)} {
		if e.Handle(k) {
			t.Fatalf("key %v must be left to the caller", k.Name())
		}
	}
	e.Insert([]rune("a\nb"))
	if e.String() != "xa b" {
		t.Fatalf("newline in a single-line field: %q", e.String())
	}
}

func TestTextEditMultiLine(t *testing.T) {
	e := textEdit{Multi: true, Width: 10}
	editType(&e, "abc")
	e.Handle(ek(tcell.KeyEnter, 0, tcell.ModAlt))
	editType(&e, "defgh")
	e.Handle(ek(tcell.KeyCtrlJ, 0, 0))
	editType(&e, "z")
	if e.String() != "abc\ndefgh\nz" {
		t.Fatalf("got %q", e.String())
	}
	e.Handle(ek(tcell.KeyUp, 0, 0))
	if e.Cursor != 5 { // z sits in column 1, so the cursor lands after the d of "defgh"
		t.Fatalf("up: cursor %d", e.Cursor)
	}
	e.Handle(ek(tcell.KeyUp, 0, 0))
	e.Handle(ek(tcell.KeyUp, 0, 0))
	if e.Cursor != 0 {
		t.Fatalf("up from the first row goes to the start: %d", e.Cursor)
	}
	e.Handle(ek(tcell.KeyEnd, 0, 0))
	if e.Cursor != 3 {
		t.Fatalf("end of the first line: %d", e.Cursor)
	}
	if e.Handle(ek(tcell.KeyEnter, 0, 0)) {
		t.Fatal("plain Enter belongs to the caller")
	}
}
