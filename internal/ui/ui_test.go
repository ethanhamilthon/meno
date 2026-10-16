package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestPadAndTruncateWideRunes(t *testing.T) {
	if got := Pad("ab", 5); got != "ab   " {
		t.Fatalf("pad %q", got)
	}
	if got := Pad("привет", 3); got != "привет" {
		t.Fatalf("pad must not cut: %q", got)
	}
	if got := Truncate("привет мир", 5); TextWidth(got) > 5 {
		t.Fatalf("truncate too wide: %q (%d)", got, TextWidth(got))
	}
	if got := Truncate("abc", 1); got != "" {
		t.Fatalf("w<=1 should give empty, got %q", got)
	}
}

func TestWrapKeepsParagraphsAndWidth(t *testing.T) {
	lines := Wrap("one two three four\n\nfive", 9)
	for _, l := range lines {
		if TextWidth(l) > 9 {
			t.Fatalf("line too wide: %q", l)
		}
	}
	if !strings.Contains(strings.Join(lines, "|"), "five") {
		t.Fatalf("lost text: %v", lines)
	}
	if got := Wrap("abc", 0); len(got) != 1 || got[0] != "abc" {
		t.Fatalf("width 0 should return input, got %v", got)
	}
}

func TestHighlightJSClassifiesTokens(t *testing.T) {
	segs := HighlightJS(`const x = "s" + 42`)
	var hasStr, hasNum bool
	for _, s := range segs {
		if s.Text == `"s"` && s.FG == Green {
			hasStr = true
		}
		if s.Text == "42" && s.FG == Orange {
			hasNum = true
		}
	}
	if !hasStr || !hasNum {
		t.Fatalf("segments %+v", segs)
	}
}

func TestModeLabels(t *testing.T) {
	if ModeInsert.Label() != "INSERT" || ModeVisual.Label() != "VISUAL" {
		t.Fatal("mode labels changed")
	}
}

func TestCodeLinesRespectMax(t *testing.T) {
	lines := CodeLines("a\nb\nc\nd", BgJSDone, 2)
	if len(lines) != 3 || lines[2].Segs[0].Text != "  …" {
		t.Fatalf("expected 2 code lines + ellipsis, got %d", len(lines))
	}
}

func TestPlainKeepsBackground(t *testing.T) {
	if l := Plain("x", tcell.ColorWhite, BgUser); l.BG != BgUser {
		t.Fatal("plain line lost background")
	}
}
