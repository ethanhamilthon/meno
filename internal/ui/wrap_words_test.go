package ui

import (
	"strings"
	"testing"
)

func TestSourceWordWrappingPreservesText(t *testing.T) {
	source := "alpha beta gamma  delta\tend"
	lines := wrapSegments([]Seg{{Text: source, FG: Green}}, 12)
	var out strings.Builder
	for _, line := range lines {
		for _, seg := range line.Segs {
			out.WriteString(seg.Text)
			if seg.FG != Green {
				t.Fatal("lost style")
			}
		}
	}
	if out.String() != source {
		t.Fatalf("lost source %q", out.String())
	}
	if got := joinSegs(lines[0].Segs); got != "alpha beta " {
		t.Fatalf("split word %q", got)
	}
}
