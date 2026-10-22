package render

import (
	"strings"
	"testing"
)

func TestMarkdownStylesAndSingleMarker(t *testing.T) {
	lines := MarkdownLines("# Title\n\n**bold** and `code`\n\n- item one\n- item two\n\n```go\nfmt.Println(1)\n```", 15, "◆")
	markers := 0
	bold, code := false, false
	var text strings.Builder
	for _, l := range lines {
		for _, s := range l.Segs {
			text.WriteString(s.Text)
			markers += strings.Count(s.Text, "◆")
			bold = bold || s.Bold
			code = code || s.FG == Blue
		}
	}
	if markers != 1 || !bold || !code {
		t.Fatalf("markers=%d bold=%v code=%v", markers, bold, code)
	}
	if strings.Contains(text.String(), "**") || strings.Contains(text.String(), "```") || !strings.Contains(text.String(), "•") {
		t.Fatalf("unrendered markdown: %s", text.String())
	}
}

func TestMarkdownLongLineWidths(t *testing.T) {
	for _, width := range []int{1, 5, 20} {
		lines := MarkdownLines(strings.Repeat("**hello** ", 30), width, "◆")
		for _, l := range lines {
			if l.Width() > width+2 {
				t.Fatalf("width %d: %d", width, l.Width())
			}
		}
	}
}

func TestMarkdownIncompleteAndEmpty(t *testing.T) {
	for _, s := range []string{"", "**", "```js\nconst a=1", "[label](https://example.com)", "你好 👩🏽‍💻"} {
		_ = MarkdownLines(s, 0, "◆")
	}
}

// ---- helpers ----

// mdLines renders and returns the text of each line without the marker.
func mdLines(src string, w int) []string {
	var out []string
	for _, l := range MarkdownLines(src, w, "◆") {
		out = append(out, strings.TrimRight(string([]rune(l.Text())[2:]), " "))
	}
	return out
}

func mdJoined(src string, w int) string { return strings.Join(mdLines(src, w), "\n") }

// findSeg returns the first segment whose text contains s.
func findSeg(t *testing.T, lines []Line, s string) Seg {
	t.Helper()
	for _, l := range lines {
		for _, g := range l.Segs {
			if strings.Contains(g.Text, s) {
				return g
			}
		}
	}
	t.Fatalf("no segment with %q in %v", s, lineTexts(lines))
	return Seg{}
}

func lineTexts(lines []Line) []string {
	var out []string
	for _, l := range lines {
		out = append(out, l.Text())
	}
	return out
}

func hasLine(lines []string, want string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

// ---- inline ----

func TestMarkdownInlineStyles(t *testing.T) {
	lines := MarkdownLines("**bold** *italic* ~~gone~~ `code` ***both***", 80, "◆")
	if s := findSeg(t, lines, "bold"); !s.Bold || s.Italic || s.FG != FgWhite {
		t.Fatalf("bold: %+v", s)
	}
	if s := findSeg(t, lines, "italic"); !s.Italic || s.Bold {
		t.Fatalf("italic: %+v", s)
	}
	if s := findSeg(t, lines, "gone"); !s.Strike || s.FG != FgDim {
		t.Fatalf("strike: %+v", s)
	}
	if s := findSeg(t, lines, "both"); !s.Bold || !s.Italic {
		t.Fatalf("bold italic: %+v", s)
	}
	code := findSeg(t, lines, "code")
	if code.FG != Orange || code.BG != BgPopup || code.Text != " code " {
		t.Fatalf("code span: %+v", code)
	}
	if got := lines[0].Text(); !strings.Contains(got, " code ") {
		t.Fatalf("code span not padded: %q", got)
	}
	for _, mark := range []string{"**", "~~", "`"} {
		if strings.Contains(lines[0].Text(), mark) {
			t.Fatalf("raw marker %q left: %q", mark, lines[0].Text())
		}
	}
}

func TestMarkdownLinks(t *testing.T) {
	cases := []struct {
		src, want, not string
	}{
		{"[docs](https://www.example.com/a)", "docs ↗ example.com", ""},
		{"[https://example.com/x](https://example.com/x)", "https://example.com/x", "↗"},
		{"[example.com](https://example.com/)", "example.com", "↗"},
		{"[local](./README.md)", "local", "↗"},
		{"[mail](mailto:a@b.c)", "mail", "↗"},
		{"[](https://example.com/p)", "https://example.com/p", "↗"},
		{"see https://example.com/a/b now", "https://example.com/a/b", "↗"},
		{"![diagram](https://x.dev/arch.png)", "▣ diagram ↗ x.dev", ""},
		{"![](./img/arch.png)", "▣ arch.png", "↗"},
		{"[bad](ht!tp://%zz)", "bad", "↗"},
	}
	for _, c := range cases {
		got := mdJoined(c.src, 80)
		if !strings.Contains(got, c.want) || c.not != "" && strings.Contains(got, c.not) {
			t.Errorf("%q → %q, want %q without %q", c.src, got, c.want, c.not)
		}
	}
	lines := MarkdownLines("[docs](https://example.com/a)", 80, "◆")
	if s := findSeg(t, lines, "docs"); s.FG != Blue || !s.Under {
		t.Fatalf("link style: %+v", s)
	}
	if s := findSeg(t, lines, "↗"); s.FG != FgDim || s.Under {
		t.Fatalf("host style: %+v", s)
	}
}

func TestMarkdownBreaks(t *testing.T) {
	got := mdLines("first line\nsecond line\n\nnew paragraph  \nhard break<br>html break <br/> x", 80)
	want := []string{"first line", "second line", "", "new paragraph", "hard break", "html break", "x"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", got)
	}
}

func TestMarkdownHTML(t *testing.T) {
	got := mdJoined("before\n\n<!-- hidden note -->\n\nafter <!-- inline --> text <b>bold</b>\n\n<div>\nblock html\n</div>", 80)
	if strings.Contains(got, "hidden") || strings.Contains(got, "inline") {
		t.Fatalf("comments shown: %q", got)
	}
	if !strings.Contains(got, "after") || !strings.Contains(got, "<b>") || !strings.Contains(got, "block html") {
		t.Fatalf("other html lost: %q", got)
	}
	if s := findSeg(t, MarkdownLines("x <b>y</b>", 80, "◆"), "<b>"); s.FG != FgDim {
		t.Fatalf("html tag style: %+v", s)
	}
}

func TestMarkdownDropsControlCharacters(t *testing.T) {
	for _, src := range []string{"a\x1b[31mred\x1b[0m b", "bell\x07 nul\x00 del\x7f c1\u0085 end", "tab\tseparated", "cr\rlf", "```\ncode\x1b[2J\n```", "| a\x1b |\n|---|\n| b\x1b |"} {
		for _, l := range MarkdownLines(src, 40, "◆") {
			for _, r := range l.Text() {
				if r < ' ' || r == 0x7f || r >= 0x80 && r < 0xa0 {
					t.Fatalf("%q: control character %q survived in %q", src, r, l.Text())
				}
			}
		}
	}
	if got := mdJoined("tab\tseparated", 40); got != "tab separated" {
		t.Fatalf("tab: %q", got)
	}
}

// ---- blocks ----

func TestMarkdownHeadings(t *testing.T) {
	lines := MarkdownLines("# Title one\n\n## Second\n\n### Third\n\n#### Fourth\n\nSetext\n======", 40, "◆")
	texts := lineTexts(lines)
	if texts[1] != "  ━━━━━━━━━" {
		t.Fatalf("H1 rule is as wide as the title: %q", texts[1])
	}
	if s := findSeg(t, lines, "Title one"); !s.Bold || s.FG != FgWhite {
		t.Fatalf("h1: %+v", s)
	}
	if s := findSeg(t, lines, "Second"); !s.Bold || s.FG != Blue {
		t.Fatalf("h2: %+v", s)
	}
	for _, name := range []string{"Third", "Fourth"} {
		if s := findSeg(t, lines, name); !s.Bold || s.FG != FgSoft {
			t.Fatalf("%s: %+v", name, s)
		}
	}
	if !strings.Contains(strings.Join(texts, "\n"), "Setext\n  ━━━━━━") {
		t.Fatalf("setext heading: %q", texts)
	}
	// The rule never outgrows the text width.
	long := MarkdownLines("# "+strings.Repeat("word ", 20), 20, "◆")
	for _, l := range long {
		if l.Width() > 22 {
			t.Fatalf("heading too wide: %d %q", l.Width(), l.Text())
		}
	}
}

func TestMarkdownBlocksAreSeparatedByBlankLines(t *testing.T) {
	got := mdLines("para one\n\n## head\n\n- item\n\npara two\n\n---\n\nlast", 40)
	want := "para one||head||• item||para two||" + strings.Repeat("─", 40) + "||last"
	if strings.Join(got, "|") != want {
		t.Fatalf("%q", got)
	}
	// No leading or trailing blank lines, whatever the input holds.
	for _, src := range []string{"\n\n\ntext\n\n\n", "   \n# h\n   \n"} {
		lines := MarkdownLines(src, 40, "◆")
		if len(lines) == 0 || strings.TrimSpace(lines[0].Text()) == "" || strings.TrimSpace(lines[len(lines)-1].Text()) == "" {
			t.Fatalf("%q → %q", src, lineTexts(lines))
		}
	}
}
