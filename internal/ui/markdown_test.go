package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
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

func TestMarkdownCodeBlockEdgeCases(t *testing.T) {
	for _, src := range []string{"```\n```", "```go\n```", "```go\nunclosed fence", "    indented code\n    more", "```\n\n\ntext\n\n\n```", "~~~py\nprint(1)\n~~~", "````md\n```go\nx\n```\n````"} {
		lines := MarkdownLines(src, 30, "◆")
		if len(lines) == 0 {
			t.Fatalf("%q rendered nothing", src)
		}
		for _, l := range lines {
			if l.Width() > 32 {
				t.Fatalf("%q: row of %d", src, l.Width())
			}
		}
	}
	// Trailing blank lines of the block are not drawn, inner ones are.
	got := mdLines("```\n\nx\n\ny\n\n\n```", 20)
	if len(got) != 4 {
		t.Fatalf("%q", got)
	}
	if got := mdJoined("    indented code\n    more", 30); !strings.Contains(got, "indented code") || !strings.Contains(got, "more") {
		t.Fatalf("indented code block: %q", got)
	}
	if got := mdLines("```\n```", 20); len(got) != 1 {
		t.Fatalf("an empty block is a single empty row: %q", got)
	}
}

func segFor(t *testing.T, segs []Seg, text string) Seg {
	t.Helper()
	for _, s := range segs {
		if strings.TrimSpace(s.Text) == text {
			return s
		}
	}
	t.Fatalf("no %q in %+v", text, segs)
	return Seg{}
}

func TestHighlightLanguages(t *testing.T) {
	cases := []struct {
		lang, line string
		checks     map[string]string // token → colour name
	}{
		{"go", "func main() { x := \"s\" + 42 } // note", map[string]string{"func": "purple", "main": "blue", "\"s\"": "green", "42": "orange", "// note": "dim"}},
		{"golang", "return nil", map[string]string{"return": "purple", "nil": "orange"}},
		{"js", "const a = await fetch('u', 1.5)", map[string]string{"const": "purple", "await": "purple", "fetch": "blue", "'u'": "green", "1.5": "orange"}},
		{"ts", "interface A { x: number } // t", map[string]string{"interface": "purple", "// t": "dim"}},
		{"tsx", "let y = null", map[string]string{"let": "purple", "null": "orange"}},
		{"python", "def f(x): return None  # c", map[string]string{"def": "purple", "f": "blue", "None": "orange", "# c": "dim"}},
		{"py", "import os", map[string]string{"import": "purple"}},
		{"rust", "fn main() { let x = 1; }", map[string]string{"fn": "purple", "let": "purple", "1": "orange"}},
		{"java", "public class A { int x = 3; }", map[string]string{"public": "purple", "class": "purple", "3": "orange"}},
		{"sql", "SELECT id FROM t WHERE x = 'a' -- c", map[string]string{"SELECT": "purple", "FROM": "purple", "'a'": "green", "-- c": "dim"}},
		{"json", "{\"key\": \"v\", \"n\": 3, \"ok\": true}", map[string]string{"\"key\"": "blue", "\"v\"": "green", "3": "orange", "true": "orange"}},
		{"yaml", "name: \"x\" # c", map[string]string{"name": "blue", "\"x\"": "green", "# c": "dim"}},
		{"yml", "  - port: 8080", map[string]string{"port": "blue", "8080": "orange"}},
		{"bash", "if [ -f $FILE ]; then echo \"hi\" # c", map[string]string{"if": "purple", "then": "purple", "$FILE": "orange", "\"hi\"": "green", "# c": "dim"}},
		{"sh", "$ go test ./...", map[string]string{"$": "green"}},
	}
	name := func(s Seg) string {
		switch s.FG {
		case Purple:
			return "purple"
		case Green:
			return "green"
		case Orange:
			return "orange"
		case Blue:
			return "blue"
		case FgDim:
			return "dim"
		}
		return "white"
	}
	for _, c := range cases {
		h := &highlighter{lang: langOf(c.lang)}
		if h.lang == nil {
			t.Fatalf("language %q unknown", c.lang)
		}
		segs := h.line(c.line, BgPopup)
		if joinText(segs) != c.line {
			t.Fatalf("%s: text changed: %q", c.lang, joinText(segs))
		}
		for tok, col := range c.checks {
			found := false
			for _, s := range segs {
				if strings.TrimSpace(s.Text) == tok {
					found = true
					if name(s) != col {
						t.Errorf("%s: %q is %s, want %s", c.lang, tok, name(s), col)
					}
				}
			}
			if !found {
				t.Errorf("%s: no segment %q in %+v", c.lang, tok, segs)
			}
		}
		for _, s := range segs {
			if s.BG != BgPopup {
				t.Fatalf("%s: segment %q off the panel", c.lang, s.Text)
			}
		}
	}
	if s := segFor(t, (&highlighter{lang: langOf("go")}).line("// c", BgPopup), "// c"); !s.Italic {
		t.Fatal("comments are italic")
	}
}

func TestHighlightMultiLineConstructs(t *testing.T) {
	h := &highlighter{lang: langOf("go")}
	l1 := h.line("x := `raw", BgPopup)
	l2 := h.line("still raw", BgPopup)
	l3 := h.line("end` + y", BgPopup)
	if segFor(t, l1, "`raw").FG != Green || segFor(t, l2, "still raw").FG != Green || segFor(t, l3, "end`").FG != Green {
		t.Fatalf("raw string: %+v %+v %+v", l1, l2, l3)
	}
	if l3[len(l3)-1].FG != FgWhite {
		t.Fatal("string ended but colour continues")
	}
	h = &highlighter{lang: langOf("c")}
	a := h.line("int a; /* start", BgPopup)
	b := h.line("middle int", BgPopup)
	c := h.line("end */ int b;", BgPopup)
	if segFor(t, a, "/* start").FG != FgDim || segFor(t, b, "middle int").FG != FgDim || segFor(t, c, "end */").FG != FgDim {
		t.Fatalf("block comment: %+v %+v %+v", a, b, c)
	}
	if segFor(t, c, "int").FG != Purple {
		t.Fatal("code after a block comment")
	}
	h = &highlighter{lang: langOf("python")}
	d := h.line("s = \"\"\"doc", BgPopup)
	e := h.line("def inside", BgPopup)
	f := h.line("\"\"\" + x", BgPopup)
	if segFor(t, e, "def inside").FG != Green || segFor(t, d, "\"\"\"doc").FG != Green || segFor(t, f, "\"\"\"").FG != Green {
		t.Fatalf("docstring: %+v %+v %+v", d, e, f)
	}
}

func TestHighlightEdgeCases(t *testing.T) {
	if langOf("") != nil || langOf("brainfuck") != nil || langOf("  Go ") == nil || langOf("GO") == nil {
		t.Fatal("language lookup")
	}
	for _, lang := range []string{"go", "js", "python", "rust", "c", "sql", "json", "yaml", "sh", ""} {
		for _, line := range []string{"", " ", "\"unterminated", "'", "`", "/*", "'\\", "日本語 := \"文字\"", "x = 1e5 0x1F 3.14.15", "$", "${", "${a", "$(", "a(", "(((", "\t\t", "#!/bin/sh", "key:", "- :", "\\"} {
			h := &highlighter{lang: langOf(lang)}
			segs := h.line(line, BgPopup)
			if joinText(segs) != line {
				t.Fatalf("%s %q: text changed to %q", lang, line, joinText(segs))
			}
		}
	}
	// SQL keywords ignore case; a shell # inside a word is not a comment.
	if segFor(t, (&highlighter{lang: langOf("sql")}).line("select * from t", BgPopup), "select").FG != Purple {
		t.Fatal("sql case")
	}
	for _, s := range (&highlighter{lang: langOf("sh")}).line("echo a#b", BgPopup) {
		if s.FG == FgDim {
			t.Fatalf("# inside a word treated as a comment: %+v", s)
		}
	}
	// Unknown languages stay plain.
	plain := MarkdownLines("```brainfuck\n+++[->+<]\n```", 30, "◆")
	if s := findSeg(t, plain, "+++"); s.FG != FgWhite {
		t.Fatalf("%+v", s)
	}
}

func TestMarkdownDiffBlock(t *testing.T) {
	src := "```diff\ndiff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,2 +1,2 @@\n ctx\n-old\n+new\n\\ No newline at end of file\n```"
	lines := MarkdownLines(src, 40, "◆")
	rowBG := func(text string) Seg {
		for _, l := range lines {
			if strings.Contains(l.Text(), text) {
				return l.Segs[len(l.Segs)-1]
			}
		}
		t.Fatalf("no row %q", text)
		return Seg{}
	}
	if rowBG("new").BG != BgJSDone || rowBG("old").BG != BgJSErr || rowBG("ctx").BG != BgPopup || rowBG("@@").BG != BgPopup {
		t.Fatal("row backgrounds")
	}
	exact := func(text string) Seg {
		for _, l := range lines {
			for _, g := range l.Segs {
				if g.Text == text {
					return g
				}
			}
		}
		t.Fatalf("no segment %q", text)
		return Seg{}
	}
	if s := exact("+"); s.FG != Green || !s.Bold {
		t.Fatalf("plus: %+v", s)
	}
	if s := exact("-"); s.FG != Red || !s.Bold {
		t.Fatalf("minus: %+v", s)
	}
	if s := findSeg(t, lines, "@@ -1,2"); s.FG != Blue {
		t.Fatalf("hunk: %+v", s)
	}
	// File headers are not mistaken for added or removed lines.
	if rowBG("--- a/x").BG != BgPopup || rowBG("+++ b/x").BG != BgPopup {
		t.Fatal("file headers tinted")
	}
	for _, l := range lines {
		if l.Width() != 42 {
			t.Fatalf("diff row width %d: %q", l.Width(), l.Text())
		}
	}
	if len(lines) != 9 {
		t.Fatalf("%q", lineTexts(lines))
	}
}

// ---- tables ----

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

func TestMarkdownEmptyInputs(t *testing.T) {
	for _, src := range []string{"", " ", "\n\n", "\t", "<!-- only a comment -->", "\x1b", "  \n  "} {
		if lines := MarkdownLines(src, 40, "◆"); len(lines) != 0 {
			t.Fatalf("%q rendered %q", src, lineTexts(lines))
		}
	}
	if len(MarkdownLines("x", 40, "")) != 1 {
		t.Fatal("empty glyph")
	}
}

func TestMarkdownLargeInputIsFast(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 3000; i++ {
		b.WriteString("- item **" + strings.Repeat("w", i%40) + "** and `code` [l](https://e.com)\n")
	}
	b.WriteString("\n| a | b |\n|---|---|\n")
	for i := 0; i < 1000; i++ {
		b.WriteString("| cell " + strings.Repeat("x", i%30) + " | v |\n")
	}
	start := time.Now()
	lines := MarkdownLines(b.String(), 80, "◆")
	if len(lines) < 3000 || time.Since(start) > 3*time.Second {
		t.Fatalf("%d lines in %v", len(lines), time.Since(start))
	}
}

func TestSegAttributesReachTheScreen(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(30, 2)
	l := Line{BG: BgTerm, Segs: []Seg{
		{Text: "a", FG: Blue, Bold: true},
		{Text: "b", FG: Blue, Italic: true},
		{Text: "c", FG: Blue, Under: true},
		{Text: "d", FG: Blue, Strike: true},
		{Text: "e", FG: Orange, BG: BgPopup},
		{Text: "f", FG: Blue},
	}}
	DrawLine(Canvas{S: s}, 0, 0, 30, l)
	want := []struct {
		attr tcell.AttrMask
		bg   tcell.Color
	}{{tcell.AttrBold, BgTerm}, {tcell.AttrItalic, BgTerm}, {tcell.AttrUnderline, BgTerm}, {tcell.AttrStrikeThrough, BgTerm}, {0, BgPopup}, {0, BgTerm}}
	for i, w := range want {
		_, _, st, _ := s.GetContent(PadX+i, 0)
		_, bg, at := st.Decompose()
		if at != w.attr || bg != w.bg {
			t.Fatalf("cell %d: attrs %v bg %v, want %v %v", i, at, bg, w.attr, w.bg)
		}
	}
}
