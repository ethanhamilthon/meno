package ui

import (
	"strings"
	"testing"
)

func segText(lines []Line) string {
	var rows []string
	for _, l := range lines {
		var b strings.Builder
		for _, s := range l.Segs {
			b.WriteString(s.Text)
		}
		rows = append(rows, b.String())
	}
	return strings.Join(rows, "\n")
}

func kindsOf(t *testing.T, src string) []string {
	t.Helper()
	toks, ok := lexJS(src)
	if !ok {
		t.Fatalf("cannot lex %q", src)
	}
	var join strings.Builder
	var out []string
	names := map[jsKind]string{jsIdent: "id", jsKeyword: "kw", jsLiteral: "lit", jsNumber: "num", jsString: "str", jsTplOpen: "tpl", jsTplText: "txt", jsTplExpr: "expr", jsTplClose: "tpl/", jsRegex: "re", jsComment: "cmt", jsPunct: "p"}
	for _, tk := range toks {
		join.WriteString(tk.Text)
		if tk.Kind != jsSpace {
			out = append(out, names[tk.Kind]+":"+tk.Text)
		}
	}
	if join.String() != src {
		t.Fatalf("tokens do not concatenate back to the source:\n%q\n%q", join.String(), src)
	}
	return out
}

func TestLexerModernSyntaxAndAmbiguities(t *testing.T) {
	cases := map[string][]string{
		"const a = 1_000.5e+3":      {"kw:const", "id:a", "p:=", "num:1_000.5e+3"},
		"x = a / b / c":             {"id:x", "p:=", "id:a", "p:/", "id:b", "p:/", "id:c"},
		"x = /ab+c/gi.test(s)":      {"id:x", "p:=", "re:/ab+c/gi", "p:.", "id:test", "p:(", "id:s", "p:)"},
		"return /[/]\\//.exec(s)":   {"kw:return", "re:/[/]\\//", "p:.", "id:exec", "p:(", "id:s", "p:)"},
		"a = b ? 'x' : \"y\\\"z\"":  {"id:a", "p:=", "id:b", "p:?", "str:'x'", "p::", "str:\"y\\\"z\""},
		"// c\nlet é = 1 /* m */":   {"cmt:// c", "kw:let", "id:é", "p:=", "num:1", "cmt:/* m */"},
		"obj.class.new()":           {"id:obj", "p:.", "id:class", "p:.", "id:new", "p:(", "p:)"},
		"`a${b + `c${d}`}e`":        {"tpl:`", "txt:a", "expr:${", "id:b", "p:+", "tpl:`", "txt:c", "expr:${", "id:d", "expr:}", "tpl/:`", "expr:}", "txt:e", "tpl/:`"},
		"async () => await f(...a)": {"kw:async", "p:(", "p:)", "p:=", "p:>", "kw:await", "id:f", "p:(", "p:.", "p:.", "p:.", "id:a", "p:)"},
		"x = {a: 1}\n/foo/.test(y)": {"id:x", "p:=", "p:{", "id:a", "p::", "num:1", "p:}", "re:/foo/", "p:.", "id:test", "p:(", "id:y", "p:)"},
	}
	for src, want := range cases {
		got := kindsOf(t, src)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%q\n got %v\nwant %v", src, got, want)
		}
	}
}

func TestLexerRejectsIncompleteSource(t *testing.T) {
	for _, src := range []string{"'open", "x = `never closed", "a /* open", "x = /re", "`a${b"} {
		if _, ok := lexJS(src); ok {
			t.Errorf("%q must fall back to plain text", src)
		}
	}
	lines := CodeLinesOpt("'open", BgJSDone, 40, 0, CodeOpts{})
	if segText(lines) != "'open" {
		t.Fatalf("fallback must stay readable: %q", segText(lines))
	}
}

func TestStringsStayStringsWhateverTheyContain(t *testing.T) {
	src := "const q = \"SELECT * FROM t WHERE a = 'x' -- <b>html</b> function(){}\""
	lines := CodeLinesOpt(src, BgJSDone, 0, 0, CodeOpts{})
	var greens int
	for _, s := range lines[0].Segs {
		if s.FG == Green {
			greens++
			if !strings.Contains(s.Text, "SELECT") || !strings.Contains(s.Text, "</b>") {
				t.Fatalf("the whole literal is one string segment: %q", s.Text)
			}
		}
	}
	if greens != 1 {
		t.Fatalf("string pieces: %d", greens)
	}
}

func TestStreamingCodeIsPlain(t *testing.T) {
	plain := CodeLinesOpt("const a = 'x'", BgJSDone, 0, 0, CodeOpts{Plain: true})
	if len(plain[0].Segs) != 1 || plain[0].Segs[0].FG != FgWhite {
		t.Fatalf("streaming fragments are not highlighted: %+v", plain[0].Segs)
	}
}
