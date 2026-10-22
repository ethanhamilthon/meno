package tui

import "testing"

func vt(lines ...string) vtext {
	var t vtext
	for _, l := range lines {
		t = append(t, []rune(l))
	}
	return t
}

func TestVimWordMotions(t *testing.T) {
	d := vt("foo bar.baz", "", "  qux")
	f := d.flat()
	step := func(fn func([]rune, int, bool) int, from vpos, big bool) vpos {
		return d.pos(fn(f, d.idx(from), big))
	}
	cases := []struct {
		name string
		fn   func([]rune, int, bool) int
		from vpos
		big  bool
		want vpos
	}{
		{"w word", wordForward, vpos{0, 0}, false, vpos{0, 4}},
		{"w punct", wordForward, vpos{0, 4}, false, vpos{0, 7}},
		{"W skips punct", wordForward, vpos{0, 4}, true, vpos{1, 0}},
		{"w stops at empty line", wordForward, vpos{0, 8}, false, vpos{1, 0}},
		{"w from empty line", wordForward, vpos{1, 0}, false, vpos{2, 2}},
		{"e", wordEnd, vpos{0, 0}, false, vpos{0, 2}},
		{"e next", wordEnd, vpos{0, 2}, false, vpos{0, 6}},
		{"b", wordBack, vpos{0, 4}, false, vpos{0, 0}},
		{"b punct", wordBack, vpos{0, 8}, false, vpos{0, 7}},
		{"b over empty line", wordBack, vpos{2, 2}, false, vpos{1, 0}},
	}
	for _, c := range cases {
		if got := step(c.fn, c.from, c.big); got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

func TestVtextIndexRoundTrip(t *testing.T) {
	d := vt("ab", "", "cde")
	for i := 0; i <= len(d.flat()); i++ {
		if got := d.idx(d.pos(i)); got != i {
			t.Fatalf("idx %d -> %+v -> %d", i, d.pos(i), got)
		}
	}
}

func TestVimMotionDispatch(t *testing.T) {
	d := vt("hello world", "", "foo(bar)", "x")
	type c struct {
		from vpos
		m    string
		a    moveArgs
		want vpos
		kind mkind
		ok   bool
	}
	cases := []c{
		{vpos{0, 3}, "h", moveArgs{n: 2}, vpos{0, 1}, mExcl, true},
		{vpos{0, 9}, "l", moveArgs{n: 9}, vpos{0, 10}, mExcl, true},
		{vpos{0, 3}, "$", moveArgs{n: 1}, vpos{0, 10}, mIncl, true},
		{vpos{0, 3}, "j", moveArgs{n: 1, want: 3}, vpos{1, 0}, mLine, true},
		{vpos{1, 0}, "j", moveArgs{n: 1, want: 5}, vpos{2, 5}, mLine, true},
		{vpos{2, 5}, "G", moveArgs{n: 1}, vpos{3, 0}, mLine, true},
		{vpos{2, 5}, "gg", moveArgs{n: 1}, vpos{0, 0}, mLine, true},
		{vpos{2, 5}, "G", moveArgs{n: 3, hasN: true}, vpos{2, 0}, mLine, true},
		{vpos{0, 0}, "fo", moveArgs{n: 1}, vpos{}, 0, false},
		{vpos{0, 0}, "f", moveArgs{n: 2, ch: 'o'}, vpos{0, 7}, mIncl, true},
		{vpos{0, 0}, "t", moveArgs{n: 1, ch: 'w'}, vpos{0, 5}, mIncl, true},
		{vpos{0, 8}, "F", moveArgs{n: 1, ch: 'o'}, vpos{0, 7}, mExcl, true},
		{vpos{0, 0}, "f", moveArgs{n: 1, ch: 'z'}, vpos{0, 0}, mExcl, false},
		{vpos{0, 2}, "}", moveArgs{n: 1}, vpos{1, 0}, mExcl, true},
		{vpos{2, 2}, "{", moveArgs{n: 1}, vpos{1, 0}, mExcl, true},
	}
	for i, c := range cases {
		if c.m == "fo" {
			continue
		}
		got, kind, ok := d.motion(c.from, c.m, c.a)
		if ok != c.ok || (ok && (got != c.want || kind != c.kind)) {
			t.Errorf("case %d %q: got %+v kind %v ok %v", i, c.m, got, kind, ok)
		}
	}
}
