package tui

import "testing"

func str(t vtext) string { return string(t.flat()) }

func TestVimCutAndSpan(t *testing.T) {
	d := vt("hello world", "second", "third")
	nt, p := d.cut(d.span(vpos{0, 0}, vpos{0, 6}, mExcl))
	if str(nt) != "world\nsecond\nthird" || p != (vpos{0, 0}) {
		t.Fatalf("%q %+v", str(nt), p)
	}
	nt, _ = d.cut(d.span(vpos{0, 6}, vpos{0, 10}, mIncl))
	if str(nt) != "hello \nsecond\nthird" {
		t.Fatalf("%q", str(nt))
	}
	nt, p = d.cut(d.span(vpos{1, 2}, vpos{1, 0}, mLine))
	if str(nt) != "hello world\nthird" || p.line != 1 {
		t.Fatalf("%q %+v", str(nt), p)
	}
	nt, p = d.cut(d.span(vpos{2, 0}, vpos{2, 0}, mLine))
	if str(nt) != "hello world\nsecond" || p.line != 1 {
		t.Fatalf("last line: %q %+v", str(nt), p)
	}
	nt, _ = d.cut(d.span(vpos{0, 0}, vpos{2, 4}, mLine))
	if str(nt) != "" || len(nt) != 1 {
		t.Fatalf("everything: %q %d", str(nt), len(nt))
	}
	// exclusive motion ending in column 0 does not eat the next line's start
	nt, _ = d.cut(d.span(vpos{0, 6}, vpos{1, 0}, mExcl))
	if str(nt) != "hello \nsecond\nthird" {
		t.Fatalf("%q", str(nt))
	}
}

func TestVimPutAndJoin(t *testing.T) {
	d := vt("abc", "xyz")
	nt, p := d.put(vpos{0, 1}, "12", false, false)
	if str(nt) != "ab12c\nxyz" || p != (vpos{0, 3}) {
		t.Fatalf("%q %+v", str(nt), p)
	}
	nt, p = d.put(vpos{0, 1}, "12", false, true)
	if str(nt) != "a12bc\nxyz" || p != (vpos{0, 2}) {
		t.Fatalf("%q %+v", str(nt), p)
	}
	nt, p = d.put(vpos{0, 1}, "L1\nL2", true, false)
	if str(nt) != "abc\nL1\nL2\nxyz" || p != (vpos{1, 0}) {
		t.Fatalf("%q %+v", str(nt), p)
	}
	nt, _ = d.put(vpos{1, 0}, "up", true, true)
	if str(nt) != "abc\nup\nxyz" {
		t.Fatalf("%q", str(nt))
	}
	nt, p = d.joinLines(0, 2)
	if str(nt) != "abc xyz" || p != (vpos{0, 3}) {
		t.Fatalf("%q %+v", str(nt), p)
	}
}
