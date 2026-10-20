package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
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

func TestUnicodeInputAndCommandArguments(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	vimKeys(a, "привет")
	if string(a.st.Input) != "привет" {
		t.Fatal(string(a.st.Input))
	}
	a.onKey(keyK(tcell.KeyEsc))
	vimKeys(a, "cd /tmp/проект")
	if string(a.st.Cmd) != "cd /tmp/проект" {
		t.Fatal(string(a.st.Cmd))
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
