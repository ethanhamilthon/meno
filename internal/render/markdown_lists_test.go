package render

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestMarkdownLists(t *testing.T) {
	got := mdLines("1. one\n2. two\n   - nested a\n     - deep\n   - nested b\n3. three", 60)
	// The first level is numbered, deeper levels use the next bullet.
	want := []string{"1. one", "2. two", "   ◦ nested a", "     ▪ deep", "   ◦ nested b", "3. three"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", got)
	}
	got = mdLines("- a\n  - b\n    - c\n      - d", 60)
	if strings.Join(got, "|") != "• a|  ◦ b|    ▪ c|      ▪ d" {
		t.Fatalf("bullets by depth: %q", got)
	}
	// Numbers are right aligned and may start anywhere.
	got = mdLines("9. nine\n10. ten\n11. eleven", 60)
	if strings.Join(got, "|") != " 9. nine|10. ten|11. eleven" {
		t.Fatalf("%q", got)
	}
	// Loose lists keep a blank line between items.
	got = mdLines("- a\n\n- b", 60)
	if strings.Join(got, "|") != "• a||• b" {
		t.Fatalf("loose: %q", got)
	}
	ord := findSeg(t, MarkdownLines("1. x", 20, "◆"), "1.")
	if ord.FG != Purple || !ord.Bold {
		t.Fatalf("number style: %+v", ord)
	}
}

func TestMarkdownListWrapsWithHangingIndent(t *testing.T) {
	got := mdLines("1. alpha beta gamma delta epsilon zeta eta theta\n   - nested alpha beta gamma delta epsilon zeta", 24)
	for i, l := range got {
		if TextWidth(l) > 24 {
			t.Fatalf("line %d too wide: %q", i, l)
		}
	}
	if len(got) < 4 || !strings.HasPrefix(got[0], "1. ") || !strings.HasPrefix(got[1], "   ") || strings.HasPrefix(got[1], "    ") && !strings.HasPrefix(got[1], "   a") {
		t.Fatalf("continuation not aligned under the text: %q", got)
	}
	for _, l := range got[1:] {
		if !strings.HasPrefix(l, "   ") {
			t.Fatalf("continuation without indent: %q", l)
		}
	}
}

func TestMarkdownTaskLists(t *testing.T) {
	lines := MarkdownLines("- [x] done thing\n- [ ] open thing\n- plain", 60, "◆")
	got := lineTexts(lines)
	if got[0] != "◆ [x] done thing" || got[1] != "  [ ] open thing" || got[2] != "  • plain" {
		t.Fatalf("%q", got)
	}
	if s := findSeg(t, lines, "[x]"); s.FG != Green {
		t.Fatalf("ticked box: %+v", s)
	}
	if s := findSeg(t, lines, "done thing"); s.FG != FgDim {
		t.Fatalf("ticked text is dimmed: %+v", s)
	}
	if s := findSeg(t, lines, "open thing"); s.FG != FgWhite {
		t.Fatalf("open text: %+v", s)
	}
	// The dimming does not leak into the next item.
	if s := findSeg(t, lines, "plain"); s.FG != FgWhite {
		t.Fatalf("leak: %+v", s)
	}
}

func TestMarkdownQuotes(t *testing.T) {
	lines := MarkdownLines("> quoted *text*\n> second\n>\n> > nested", 40, "◆")
	got := lineTexts(lines)
	want := []string{"◆ ▎ quoted text", "  ▎ second", "  ▎ ", "  ▎ ▎ nested"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%q", got)
	}
	if s := findSeg(t, lines, "nested"); !s.Italic || s.FG != FgSoft {
		t.Fatalf("quote text: %+v", s)
	}
	if s := findSeg(t, lines, "▎"); s.FG != FgDim {
		t.Fatalf("bar: %+v", s)
	}
	// Every wrapped line carries the bar.
	for _, l := range mdLines("> "+strings.Repeat("word ", 30), 30) {
		if !strings.HasPrefix(l, "▎ ") {
			t.Fatalf("wrapped quote line without bar: %q", l)
		}
	}
	// A quote may hold other blocks.
	got2 := mdLines("> - a\n> - b\n>\n> ```go\n> x := 1\n> ```", 40)
	if !hasLine(got2, "▎ • a") || !strings.Contains(strings.Join(got2, "\n"), "▎  x := 1") {
		t.Fatalf("%q", got2)
	}
}

func TestMarkdownCallouts(t *testing.T) {
	cases := map[string]string{"NOTE": "blue", "TIP": "green", "IMPORTANT": "purple", "WARNING": "yellow", "CAUTION": "red"}
	for kind := range cases {
		lines := MarkdownLines("> [!"+kind+"]\n> body text here", 40, "◆")
		got := lineTexts(lines)
		if got[0] != "◆ ▎ ● "+kind || got[1] != "  ▎ body text here" || len(got) != 2 {
			t.Fatalf("%s: %q", kind, got)
		}
		label := findSeg(t, lines, kind)
		bar := findSeg(t, lines, "▎")
		if !label.Bold || label.FG != bar.FG || label.FG == FgDim {
			t.Fatalf("%s colour: label %+v bar %+v", kind, label, bar)
		}
		if body := findSeg(t, lines, "body"); body.Italic {
			t.Fatalf("callout body is not italic: %+v", body)
		}
	}
	colors := map[string]tcell.Color{}
	for kind := range cases {
		colors[kind] = findSeg(t, MarkdownLines("> [!"+kind+"]\n> x", 40, "◆"), kind).FG
	}
	if colors["WARNING"] != Yellow || colors["CAUTION"] != Red || colors["TIP"] != Green || colors["NOTE"] != Blue || colors["IMPORTANT"] != Purple {
		t.Fatalf("%v", colors)
	}
	// Lower case tags work; so does a tag with nothing after it, and several lines.
	if got := mdLines("> [!note]\n> a\n> b", 40); strings.Join(got, "|") != "▎ ● NOTE|▎ a|▎ b" {
		t.Fatalf("%q", got)
	}
	if got := mdLines("> [!TIP]", 40); len(got) != 1 || got[0] != "▎ ● TIP" {
		t.Fatalf("%q", got)
	}
	// Formatting inside the body is kept and a second paragraph follows.
	got := mdLines("> [!WARNING]\n> a **b**\n>\n> second para", 40)
	if strings.Join(got, "|") != "▎ ● WARNING|▎ a b|▎|▎ second para" {
		t.Fatalf("%q", got)
	}
}

func TestMarkdownNotACallout(t *testing.T) {
	for _, src := range []string{"> [!NOTE] same line", "> [!UNKNOWN]\n> x", "> text [!NOTE]\n> x", "> [!NOTE] x\n> y", "[!NOTE]\n> x"} {
		got := mdJoined(src, 60)
		if strings.Contains(got, "● ") {
			t.Errorf("%q treated as a callout: %q", src, got)
		}
	}
	if got := mdJoined("> [!UNKNOWN]\n> x", 60); !strings.Contains(got, "[!UNKNOWN]") {
		t.Fatalf("text lost: %q", got)
	}
}

// ---- code ----

func TestMarkdownCodeBlockLayout(t *testing.T) {
	src := "```go internal/agent/queue.go\nfunc f() {\n\treturn\n}\n```"
	lines := MarkdownLines(src, 40, "◆")
	got := lineTexts(lines)
	if len(got) != 4 || !strings.HasPrefix(got[0], "◆  go  internal/agent/queue.go") {
		t.Fatalf("%q", got)
	}
	if strings.Contains(strings.Join(got, ""), "\t") || !strings.Contains(got[2], "     return") {
		t.Fatalf("tab not expanded: %q", got[2])
	}
	// Short blocks have no line numbers.
	if strings.Contains(got[1], "1") {
		t.Fatalf("numbers on a short block: %q", got[1])
	}
	// The panel is as wide as the text on every row.
	for i, l := range lines {
		if l.Width() != 42 {
			t.Fatalf("row %d is %d wide: %q", i, l.Width(), l.Text())
		}
		if i > 0 && l.Segs[len(l.Segs)-1].BG != BgPopup {
			t.Fatalf("row %d not on the panel", i)
		}
	}
	if s := findSeg(t, lines, "go"); s.FG != Blue || !s.Bold {
		t.Fatalf("language: %+v", s)
	}
	if s := findSeg(t, lines, "queue.go"); s.FG != FgDim {
		t.Fatalf("file: %+v", s)
	}
}

func TestMarkdownCodeBlockLineNumbers(t *testing.T) {
	var b strings.Builder
	b.WriteString("```go\n")
	for i := 1; i <= 12; i++ {
		b.WriteString("x := " + strings.Repeat("1", i%3+1) + "\n")
	}
	b.WriteString("```")
	got := mdLines(b.String(), 40)
	if !strings.HasPrefix(got[1], "  1  x :=") || !strings.HasPrefix(got[10], " 10  x :=") || !strings.HasPrefix(got[12], " 12  x :=") {
		t.Fatalf("%q", got)
	}
	five := mdJoined("```go\na\nb\nc\nd\ne\n```", 40)
	if strings.Contains(five, " 1  ") {
		t.Fatalf("5 lines are not numbered: %q", five)
	}
	six := mdJoined("```\na\nb\nc\nd\ne\nf\n```", 40)
	if !strings.Contains(six, "1  a") || !strings.Contains(six, "6  f") {
		t.Fatalf("6 lines are numbered even without a language: %q", six)
	}
}

func TestMarkdownCodeBlockWrapsAndKeepsSpacing(t *testing.T) {
	src := "```\n" + strings.Repeat("0123456789", 5) + "\n    indented   spaces\n```"
	lines := MarkdownLines(src, 24, "◆")
	for i, l := range lines {
		if l.Width() != 26 {
			t.Fatalf("row %d: width %d", i, l.Width())
		}
	}
	got := lineTexts(lines)
	if !strings.Contains(strings.ReplaceAll(strings.Join(got, ""), " ", ""), strings.Repeat("0123456789", 5)) {
		t.Fatalf("long line lost characters: %q", got)
	}
	if !strings.Contains(got[len(got)-1], "     indented   spaces") {
		t.Fatalf("spacing changed: %q", got[len(got)-1])
	}
	// Wrapped numbered blocks show the number only on the first row.
	numbered := "```\n1\n2\n3\n4\n5\n" + strings.Repeat("w", 40) + "\n```"
	rows := mdLines(numbered, 20)
	count := 0
	for _, r := range rows {
		if strings.HasPrefix(strings.TrimSpace(r), "6") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("%q", rows)
	}
}
