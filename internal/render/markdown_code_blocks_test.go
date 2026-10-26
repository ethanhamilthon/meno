package render

import (
	"strings"
	"testing"
)

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
	if rowBG("new").BG != BgDiffAdd || rowBG("old").BG != BgDiffDel || rowBG("ctx").BG != BgPopup || rowBG("@@").BG != BgPopup {
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
