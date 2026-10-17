package app

import "testing"

func TestVimObjects(t *testing.T) {
	d := vt("say foo(bar, [1,2]) \"hi there\" ok", "", "para one", "para two", "", "tail")
	cases := []struct {
		p    vpos
		key  string
		want string
	}{
		{vpos{0, 1}, "iw", "say"},
		{vpos{0, 1}, "aw", "say "},
		{vpos{0, 3}, "iw", " "},
		{vpos{0, 6}, "iw", "foo"},
		{vpos{0, 9}, "i(", "bar, [1,2]"},
		{vpos{0, 9}, "a(", "(bar, [1,2])"},
		{vpos{0, 15}, "i[", "1,2"},
		{vpos{0, 25}, "i\"", "hi there"},
		{vpos{0, 25}, "a\"", "\"hi there\""},
		{vpos{0, 10}, "ib", "bar, [1,2]"},
		{vpos{2, 2}, "ip", "para one\npara two"},
	}
	for _, c := range cases {
		r, ok := d.object(c.p, c.key)
		if !ok || d.slice(r) != c.want {
			t.Errorf("%+v %s: %q ok=%v want %q", c.p, c.key, d.slice(r), ok, c.want)
		}
	}
	if _, ok := d.object(vpos{5, 0}, "i("); ok {
		t.Error("no enclosing pair expected")
	}
	e := vt("f()")
	if r, ok := e.object(vpos{0, 1}, "i("); !ok || r.lo != r.hi {
		t.Errorf("empty pair: %+v %v", r, ok)
	}
}
