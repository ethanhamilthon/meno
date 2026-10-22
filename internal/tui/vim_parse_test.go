package tui

import "testing"

func TestParseVim(t *testing.T) {
	cases := []struct {
		in     string
		visual bool
		state  parseState
		n      int
		op     rune
		key    string
		ch     rune
		opLine bool
	}{
		{"", false, parseMore, 1, 0, "", 0, false},
		{"3", false, parseMore, 3, 0, "", 0, false},
		{"w", false, parseDone, 1, 0, "w", 0, false},
		{"12j", false, parseDone, 12, 0, "j", 0, false},
		{"0", false, parseDone, 1, 0, "0", 0, false},
		{"d", false, parseMore, 1, 'd', "", 0, false},
		{"dd", false, parseDone, 1, 'd', "d", 0, true},
		{"2d3w", false, parseDone, 6, 'd', "w", 0, false},
		{"dfx", false, parseDone, 1, 'd', "f", 'x', false},
		{"df", false, parseMore, 1, 'd', "", 0, false},
		{"ciw", false, parseDone, 1, 'c', "iw", 0, false},
		{"ci", false, parseMore, 1, 'c', "", 0, false},
		{"da\"", false, parseDone, 1, 'd', "a\"", 0, false},
		{"gg", false, parseDone, 1, 0, "gg", 0, false},
		{"dgg", false, parseDone, 1, 'd', "gg", 0, false},
		{"gz", false, parseInvalid, 1, 0, "", 0, false},
		{"rx", false, parseDone, 1, 0, "r", 'x', false},
		{"3rx", false, parseDone, 3, 0, "r", 'x', false},
		{"iw", true, parseDone, 1, 0, "iw", 0, false},
		{"i", false, parseDone, 1, 0, "i", 0, false},
		{"y", true, parseDone, 1, 0, "y", 0, false},
	}
	for _, c := range cases {
		got, st := parseVim([]rune(c.in), c.visual)
		if st != c.state {
			t.Errorf("%q: state %v want %v", c.in, st, c.state)
			continue
		}
		if st == parseDone && (got.n != c.n || got.op != c.op || got.key != c.key || got.ch != c.ch || got.opLine != c.opLine) {
			t.Errorf("%q: %+v", c.in, got)
		}
		if st == parseMore && (got.n != c.n || got.op != c.op) {
			t.Errorf("%q: partial %+v", c.in, got)
		}
	}
}
