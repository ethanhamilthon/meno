package render

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// colorOf returns the color of the first segment whose text is exactly word.
func colorOf(t *testing.T, src, word string) tcell.Color {
	t.Helper()
	for _, s := range bashSegments(src) {
		if s.Text == word {
			return s.FG
		}
	}
	t.Fatalf("no segment %q in %q: %v", word, src, bashSegments(src))
	return 0
}

func TestBashRolesColors(t *testing.T) {
	cases := []struct {
		src, word string
		want      func() tcell.Color
	}{
		{"git log --oneline | head -3", "git", func() tcell.Color { return FgWhite }},
		{"git log --oneline | head -3", "log", func() tcell.Color { return FgSoft }},
		{"git log --oneline | head -3", "--oneline", func() tcell.Color { return FgSoft }},
		{"git log --oneline | head -3", "|", func() tcell.Color { return FgFaint }},
		{"git log --oneline | head -3", "head", func() tcell.Color { return FgWhite }},
		{"git log --oneline | head -3", "-3", func() tcell.Color { return FgSoft }},
		{"a; b", ";", func() tcell.Color { return FgFaint }},
		{"a; b", "b", func() tcell.Color { return FgWhite }},
		{"a && b", "&&", func() tcell.Color { return FgFaint }},
		{"FOO=1 make -j4", "make", func() tcell.Color { return FgWhite }},
		{"FOO=1 make -j4", "-j4", func() tcell.Color { return FgSoft }},
		{`echo "hi $x"`, "echo", func() tcell.Color { return FgWhite }},
		{`echo "hi $x"`, "hi ", func() tcell.Color { return Green }},
		{`echo "hi $x"`, "$x", func() tcell.Color { return Orange }},
		{"x=$(cat f)", "cat", func() tcell.Color { return FgWhite }},
		{"x=$(cat f)", "f", func() tcell.Color { return FgSoft }},
		{"cd /tmp\nls -la", "ls", func() tcell.Color { return FgWhite }},
		{"cd /tmp\nls -la", "/tmp", func() tcell.Color { return FgSoft }},
		{"if a; then b; fi", "b", func() tcell.Color { return FgWhite }},
		{"ls # note", "# note", func() tcell.Color { return FgDim }},
	}
	for _, c := range cases {
		if got := colorOf(t, c.src, c.word); got != c.want() {
			t.Errorf("%q in %q: color %v, want %v", c.word, c.src, got, c.want())
		}
	}
}

func TestBashRolesRoundTrip(t *testing.T) {
	for _, src := range []string{
		"", "   ", "ls", "a;b|c&&d||e", "FOO=1 BAR='x y' cmd --flag=\"v\"", "echo `date` $(( 1 + 2 ))",
		"for i in 1 2; do echo $i; done", "cat <<EOF\nhi $x\nEOF\nls", "echo 'unterminated", "case x in a) b;; esac",
		"{ a; b; }", "a \\\n  b", "世界 --флаг | grep é", "x=1\ny=2\n",
	} {
		var got strings.Builder
		for _, s := range bashSegments(src) {
			got.WriteString(s.Text)
		}
		if got.String() != src {
			t.Errorf("round trip changed %q -> %q", src, got.String())
		}
	}
}

func TestFaintIsDarkerThanDim(t *testing.T) {
	if FgFaint == FgDim || FgFaint == BgTerm {
		t.Fatalf("faint %v dim %v bg %v", FgFaint, FgDim, BgTerm)
	}
	lum := func(c tcell.Color) int32 { r, g, b := c.RGB(); return r + g + b }
	if !(lum(FgFaint) < lum(FgDim) && lum(FgFaint) > lum(BgTerm)) {
		t.Fatal("faint must sit between dim and the background")
	}
}

func TestBashHeaderIsFaint(t *testing.T) {
	l := bashHeader(Turn{Title: "List files", ElapsedMs: 1200}, 60, JSOpts{})
	for _, s := range l.Segs {
		if s.FG != FgFaint {
			t.Fatalf("header segment %q uses %v", s.Text, s.FG)
		}
	}
}
