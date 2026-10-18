package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

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
