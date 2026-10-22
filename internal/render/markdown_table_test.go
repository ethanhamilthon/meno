package render

import (
	"os"
	"strings"
	"testing"
)

func TestMarkdownTable(t *testing.T) {
	src := "| mode | time | note |\n|:-----|-----:|:----:|\n| fast | 3.2s | ok |\n| race | 1m 52s | slow |"
	got := mdLines(src, 60)
	want := []string{
		"┌──────┬────────┬──────┐",
		"│ mode │   time │ note │",
		"├──────┼────────┼──────┤",
		"│ fast │   3.2s │  ok  │",
		"│ race │ 1m 52s │ slow │",
		"└──────┴────────┴──────┘",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s", strings.Join(got, "\n"))
	}
	lines := MarkdownLines(src, 60, "◆")
	if s := findSeg(t, lines, "mode"); !s.Bold {
		t.Fatalf("header: %+v", s)
	}
	if s := findSeg(t, lines, "fast"); s.Bold {
		t.Fatalf("body: %+v", s)
	}
	if s := findSeg(t, lines, "┌"); s.FG != FgDim {
		t.Fatalf("border: %+v", s)
	}
}

func TestMarkdownTableCellsKeepInlineStyle(t *testing.T) {
	lines := MarkdownLines("| a | b |\n|---|---|\n| `x` | **y** [l](https://e.com) |", 60, "◆")
	if s := findSeg(t, lines, "x"); s.FG != Orange || s.BG != BgPopup {
		t.Fatalf("code in cell: %+v", s)
	}
	if s := findSeg(t, lines, "y"); !s.Bold {
		t.Fatalf("bold in cell: %+v", s)
	}
	if got := mdJoined("| a |\n|---|\n| one<br>two |", 40); !strings.Contains(got, "│ one │") || !strings.Contains(got, "│ two │") {
		t.Fatalf("br in cell: %q", got)
	}
}

func TestMarkdownTableShrinksAndWraps(t *testing.T) {
	src := "| name | description |\n|---|---|\n| alpha | a very long description that needs to wrap over several lines |\n| b | short |"
	for _, w := range []int{60, 40, 30, 24, 20} {
		lines := MarkdownLines(src, w, "◆")
		widths := map[int]bool{}
		for _, l := range lines {
			widths[l.Width()] = true
			if l.Width() > w+2 {
				t.Fatalf("width %d: row of %d: %q", w, l.Width(), l.Text())
			}
		}
		text := strings.Join(lineTexts(lines), "\n")
		words := []string{"alpha", "short"}
		if w >= 30 {
			words = append(words, "description", "several") // wide enough to keep words whole
		}
		for _, word := range words {
			if !strings.Contains(text, word) {
				t.Fatalf("width %d lost %q:\n%s", w, word, text)
			}
		}
		if strings.Contains(text, "┌") && len(widths) != 1 {
			t.Fatalf("width %d: ragged table %v:\n%s", w, widths, text)
		}
	}
	// Wrapped rows are told apart by a rule between them.
	narrow := mdJoined(src, 30)
	if strings.Count(narrow, "├") != 2 {
		t.Fatalf("header rule and one row rule expected:\n%s", narrow)
	}
	// A table that fits has no row rules.
	if strings.Count(mdJoined(src, 80), "├") != 1 {
		t.Fatal("row rules on a table that fits")
	}
}

func TestMarkdownTableFallsBackWhenItCannotFit(t *testing.T) {
	src := "| one | two | three | four |\n|---|---|---|---|\n| aaa | bbb | ccc | ddd |"
	for _, w := range []int{1, 5, 12, 16} {
		lines := MarkdownLines(src, w, "◆")
		if len(lines) == 0 {
			t.Fatalf("width %d: nothing", w)
		}
		text := strings.ReplaceAll(strings.Join(lineTexts(lines), ""), " ", "")
		for _, word := range []string{"one", "four", "aaa", "ddd"} {
			if !strings.Contains(text, word) {
				t.Fatalf("width %d lost %q: %q", w, word, text)
			}
		}
		for _, l := range lines {
			if l.Width() > w+2 {
				t.Fatalf("width %d: row of %d: %q", w, l.Width(), l.Text())
			}
		}
	}
}

func TestMarkdownTableEdgeCases(t *testing.T) {
	cases := []string{
		"| a |\n|---|",
		"| a | b |\n|---|---|\n| only |",
		"| a | b |\n|---|---|\n| 1 | 2 | 3 |",
		"| a | b |\n|---|---|\n|  |  |",
		"| 日本語 | 中文 |\n|---|---|\n| テスト | 值 |",
		"| a | b |\n|---|---|\n| | x |",
		"a | b\n--|--\n1 | 2",
		"| a | b\n|---|---",
		"| a |\n|-",
	}
	for _, src := range cases {
		for _, w := range []int{1, 6, 14, 40} {
			lines := MarkdownLines(src, w, "◆")
			for _, l := range lines {
				if l.Width() > w+3 {
					t.Fatalf("%q at %d: row of %d: %q", src, w, l.Width(), l.Text())
				}
			}
		}
	}
	// Wide characters count as two cells when aligning.
	lines := MarkdownLines("| 日本 | b |\n|---|---|\n| ab | c |", 40, "◆")
	for _, l := range lines {
		if l.Width() != lines[0].Width() {
			t.Fatalf("CJK table is ragged: %q", lineTexts(lines))
		}
	}
	// A table inside a list item keeps the indent.
	got := mdLines("- item\n\n  | a | b |\n  |---|---|\n  | 1 | 2 |", 40)
	if !hasLine(got, "┌───┬───┐") || !strings.HasPrefix(got[2], "  ┌") {
		t.Fatalf("%q", got)
	}
}

// ---- wrapping and robustness ----

func TestMarkdownWrapsAtWordsNotInsideThem(t *testing.T) {
	got := mdLines("alpha beta gamma delta epsilon zeta", 12)
	want := []string{"alpha beta", "gamma delta", "epsilon zeta"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", got)
	}
	// Styled words glued to punctuation stay together.
	got = mdLines("see **bold**, then more words here", 11)
	if got[0] != "see bold," || got[1] != "then more" {
		t.Fatalf("%q", got)
	}
	// A word longer than a line is split, and nothing is lost.
	got = mdLines("short "+strings.Repeat("x", 25)+" tail", 10)
	if strings.ReplaceAll(strings.Join(got, ""), " ", "") != "short"+strings.Repeat("x", 25)+"tail" || got[0] != "short" {
		t.Fatalf("%q", got)
	}
	// Spaces at the edges of lines disappear.
	for _, l := range mdLines("one two   three    four", 8) {
		if l != strings.TrimSpace(l) {
			t.Fatalf("edge space in %q", l)
		}
	}
}

func TestMarkdownWrapsWideCharacters(t *testing.T) {
	got := mdLines(strings.Repeat("日本語", 10), 10)
	for _, l := range got {
		if TextWidth(l) > 10 {
			t.Fatalf("%q is %d wide", l, TextWidth(l))
		}
	}
	if strings.Join(got, "") != strings.Repeat("日本語", 10) || len(got) != 6 {
		t.Fatalf("%q", got)
	}
	// Even a width of one cell makes progress with two-cell characters.
	if got := mdLines("日本", 1); len(got) != 2 {
		t.Fatalf("%q", got)
	}
	// Emoji with joiners are not torn apart.
	if got := mdJoined("a 👩🏽‍💻 b", 20); got != "a 👩🏽‍💻 b" {
		t.Fatalf("%q", got)
	}
}

func TestMarkdownNeverExceedsTheWidth(t *testing.T) {
	sample, err := os.ReadFile("testdata/markdown_sample.md")
	if err != nil {
		t.Fatal(err)
	}
	for w := 12; w <= 120; w += 3 {
		for _, l := range MarkdownLines(string(sample), w, "◆") {
			if l.Width() > w+2 {
				t.Fatalf("width %d: row of %d: %q", w, l.Width(), l.Text())
			}
		}
	}
	for w := 0; w < 12; w++ {
		if len(MarkdownLines(string(sample), w, "◆")) == 0 {
			t.Fatalf("width %d: nothing", w)
		}
	}
}

func TestMarkdownEveryPrefixRenders(t *testing.T) {
	// Text arrives piece by piece: every prefix must render without trouble.
	sample, err := os.ReadFile("testdata/markdown_sample.md")
	if err != nil {
		t.Fatal(err)
	}
	rs := []rune(string(sample))
	for n := 0; n <= len(rs); n += 5 {
		for _, l := range MarkdownLines(string(rs[:n]), 50, "◆") {
			if l.Width() > 52 {
				t.Fatalf("prefix %d: row of %d: %q", n, l.Width(), l.Text())
			}
		}
	}
}

func TestMarkdownSingleMarker(t *testing.T) {
	sample, _ := os.ReadFile("testdata/markdown_sample.md")
	lines := MarkdownLines(string(sample), 60, "◇")
	if !strings.HasPrefix(lines[0].Text(), "◇ ") {
		t.Fatal("first line has no marker")
	}
	for i, l := range lines[1:] {
		if strings.HasPrefix(l.Text(), "◇") {
			t.Fatalf("line %d repeats the marker", i+1)
		}
		if !strings.HasPrefix(l.Text(), "  ") {
			t.Fatalf("line %d not indented: %q", i+1, l.Text())
		}
	}
}
