package tui

import "github.com/gdamore/tcell/v2"

// Keys that act as commands (list navigation, shortcuts of the
// dialogs, the command name after "/") are matched by their position on the
// keyboard, not by the letter the layout prints on them. So on a Russian or
// Ukrainian layout "о" is the "j" key. Text typed into the composer, search
// boxes, forms and command arguments is never translated.
var layoutKeys = func() map[rune]rune {
	m := map[rune]rune{}
	pairs := [][2]string{
		{"йцукенгшщзхъфывапролджэячсмитьбюё", "qwertyuiop[]asdfghjkl;'zxcvbnm,.`"},
		{"ЙЦУКЕНГШЩЗХЪФЫВАПРОЛДЖЭЯЧСМИТЬБЮЁ", "QWERTYUIOP{}ASDFGHJKL:\"ZXCVBNM<>~"},
		{"іїєґІЇЄҐ", "s]'\\S}\"|"}, // Ukrainian letters that sit where ы ъ э \ are
	}
	for _, p := range pairs {
		from, to := []rune(p[0]), []rune(p[1])
		for i := range from {
			m[from[i]] = to[i]
		}
	}
	return m
}()

// latinKey returns the Latin key at the position of r on the keyboard, or r
// itself when it is not a letter of a supported layout.
func latinKey(r rune) rune {
	if l, ok := layoutKeys[r]; ok {
		return l
	}
	return r
}

// isKey reports whether k is the key that types the Latin character r, on any
// supported layout.
func isKey(k *tcell.EventKey, r rune) bool {
	got := k.Rune()
	return got == r || (got != 0 && latinKey(got) == r)
}

// normalizeCtrl turns Ctrl+<letter of another layout> (reported by terminals
// with the kitty keyboard protocol) into the Ctrl key of the same position.
func normalizeCtrl(k *tcell.EventKey) *tcell.EventKey {
	if k.Key() != tcell.KeyRune || k.Modifiers()&tcell.ModCtrl == 0 {
		return k
	}
	l := latinKey(k.Rune())
	if l == k.Rune() || l < 'a' || l > 'z' {
		return k
	}
	return tcell.NewEventKey(tcell.KeyCtrlA+tcell.Key(l-'a'), 0, k.Modifiers()&^tcell.ModCtrl)
}
