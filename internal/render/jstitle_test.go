package render

import "testing"

func TestWithTitleUsesTheLastRowWhenItFits(t *testing.T) {
	rows := []Line{Plain("  a = 1", FgDim, BgTerm), Plain("  b = 2", FgDim, BgTerm)}
	out := withTitle(rows, "Set values", 40, Band(BgTerm))
	if len(out) != 2 || len(out[0].Aside) != 0 || len(out[1].Aside) != 1 || out[1].Aside[0].Text != "Set values" {
		t.Fatalf("%+v", out)
	}
}

func TestWithTitleGetsItsOwnRowWhenCrowded(t *testing.T) {
	rows := []Line{Plain("  a long line of code here", FgDim, BgTerm)}
	out := withTitle(rows, "Set values", 30, Band(BgTerm))
	if len(out) != 2 || len(out[1].Aside) != 1 {
		t.Fatalf("%+v", out)
	}
	// A row that already carries the timing is not shared.
	rows = []Line{{Segs: []Seg{{Text: "x"}}, Aside: []Seg{{Text: "✓ 1s"}}}}
	if out := withTitle(rows, "t", 80, Band(BgTerm)); len(out) != 2 {
		t.Fatalf("%+v", out)
	}
	if out := withTitle(rows, "", 80, Band(BgTerm)); len(out) != 1 {
		t.Fatal("no title, no row")
	}
}
