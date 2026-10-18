package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func runeKey(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

func TestLayoutTableIsComplete(t *testing.T) {
	for r, want := range map[rune]rune{'о': 'j', 'л': 'k', 'Ж': ':', 'й': 'q', 'ё': '`', 'х': '[', 'ъ': ']', 'і': 's', 'Я': 'Z', 'a': 'a', ' ': ' '} {
		if got := latinKey(r); got != want {
			t.Errorf("latinKey(%q) = %q, want %q", r, got, want)
		}
	}
	if len(layoutKeys) != 33*2+8 {
		t.Errorf("unexpected table size %d", len(layoutKeys))
	}
	if !isKey(runeKey('о'), 'j') || !isKey(runeKey('j'), 'j') || isKey(runeKey('к'), 'j') {
		t.Error("isKey")
	}
}

func TestRussianLayoutMovesInNormalModeButTypesRussianInInsertMode(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	for _, r := range "привет мир" {
		a.onKey(runeKey(r))
	}
	if got := string(a.st.Input); got != "привет мир" {
		t.Fatalf("insert mode must keep Russian text, got %q", got)
	}
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Mode != ui.ModeNormal {
		t.Fatal("not in normal mode")
	}
	a.onKey(runeKey('0'))
	col := a.st.Cursor
	a.onKey(runeKey('д')) // l
	a.onKey(runeKey('д'))
	if a.st.Cursor != col+2 {
		t.Fatalf("д (l) did not move right: %d -> %d", col, a.st.Cursor)
	}
	a.onKey(runeKey('р')) // h
	if a.st.Cursor != col+1 {
		t.Fatalf("р (h) did not move left: %d", a.st.Cursor)
	}
	// r replaces with a literal character: the Russian letter must stay Russian.
	a.onKey(runeKey('к'))
	a.onKey(runeKey('ы'))
	if got := string(a.st.Input); got != "пыивет мир" {
		t.Fatalf("rы must put ы, got %q", got)
	}
}

func TestCommandNameIsTranslatedButArgumentsAreNot(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	a.onKey(keyK(tcell.KeyEsc))
	a.onKey(runeKey('Ж')) // ":" on the Russian layout
	if a.st.Mode != ui.ModeCommand {
		t.Fatalf("Ж did not open the command line, mode %v", a.st.Mode)
	}
	for _, r := range "св " {
		a.onKey(runeKey(r))
	}
	for _, r := range "папка" {
		a.onKey(runeKey(r))
	}
	if got := string(a.st.Cmd); got != "cd папка" {
		t.Fatalf("got %q", got)
	}
}

func TestCtrlOnAnotherLayoutIsTheSameShortcut(t *testing.T) {
	k := normalizeCtrl(tcell.NewEventKey(tcell.KeyRune, 'с', tcell.ModCtrl))
	if k.Key() != tcell.KeyCtrlC {
		t.Fatalf("Ctrl+с should be Ctrl+C, got key %v", k.Key())
	}
	same := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModCtrl)
	if normalizeCtrl(same) != same {
		t.Fatal("a Latin key must pass through")
	}
}
