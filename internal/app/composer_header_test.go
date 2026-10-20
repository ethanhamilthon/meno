package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/ui"
)

func TestComposerHeaderDrawsTitleAtRight(t *testing.T) {
	a, screen := visualFixture(t, 88, 12)
	a.st.Session.Title = "session title"
	a.draw()
	row := visualRow(screen, 12-a.bottomHeight())
	if !strings.HasSuffix(row, "session title──") {
		t.Fatalf("name must end before the right gutter: %q", row)
	}
	cells := []rune(row)
	if got := string(cells[:88-ui.Gutter-ui.TextWidth("session title")-1]); strings.Trim(got, "─") != "" {
		t.Fatalf("rule contains unexpected cells: %q", got)
	}
	for x := 0; x < 88; x++ {
		glyph, _, style, _ := screen.GetContent(x, 12-a.bottomHeight())
		fg, bg, _ := style.Decompose()
		if bg != ui.BgTerm {
			t.Fatalf("header cell %d background %v", x, bg)
		}
		nameStart := 88 - ui.Gutter - ui.TextWidth("session title")
		if glyph != ' ' && fg != ui.FgDim {
			t.Fatalf("cell %d foreground %v", x, fg)
		}
		if x < nameStart-1 && glyph != '─' {
			t.Fatalf("header cell %d = %q, want rule", x, glyph)
		}
	}
}

func TestComposerHeaderTruncatesAtThirtyPercent(t *testing.T) {
	a, screen := visualFixture(t, 88, 12)
	a.st.Session.Title = strings.Repeat("long session name ", 8)
	a.draw()
	row := visualRow(screen, 12-a.bottomHeight())
	name := strings.Trim(strings.TrimLeft(row, "─"), "─ ")
	if !strings.HasSuffix(name, "…") {
		t.Fatalf("truncated name has no ellipsis: %q", row)
	}
	if width := ui.TextWidth(name); width > 88*composerHeaderMax/100 {
		t.Fatalf("name width %d exceeds 30%% cap", width)
	}
}

func TestComposerHeaderUsesFolderWithoutTitle(t *testing.T) {
	a, screen := visualFixture(t, 80, 10)
	a.st.Session.Replace(nil)
	a.st.Session.Title = ""
	a.st.Cwd = filepath.Join(t.TempDir(), "project")
	a.draw()
	if !strings.HasSuffix(visualRow(screen, 10-a.bottomHeight()), "project──") {
		t.Fatal("empty session did not show its folder")
	}
	a.st.Session.Title = "untitled"
	if got := a.composerHeaderLabel(); got != "project" {
		t.Fatalf("untitled label %q", got)
	}
	// The daemon names a session after its first message and sends the name.
	a.st.Session.Title = "first user message"
	if got := a.composerHeaderLabel(); got != "first user message" {
		t.Fatalf("first-message label %q", got)
	}
}

func TestComposerHeaderHomeAndNarrowWidth(t *testing.T) {
	a, screen := visualFixture(t, 80, 10)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	a.st.Session.Title = "untitled"
	a.st.Cwd = home
	if got := a.composerHeaderLabel(); got != "~" {
		t.Fatalf("home label %q", got)
	}
	a.st.Cwd = string(filepath.Separator)
	if got := a.composerHeaderLabel(); got != "~" {
		t.Fatalf("root label %q", got)
	}
	a.st.Session.Title = strings.Repeat("wide title", 4)
	a.scr.SetSize(3, 10)
	a.draw()
	row := visualRow(screen, 10-a.bottomHeight())
	if row != "───" {
		t.Fatalf("narrow header %q", row)
	}
}

func TestComposerHeightIncludesHeaderWithoutGrowing(t *testing.T) {
	a, screen := visualFixture(t, 80, 20)
	cases := []struct {
		input string
		want  int
	}{
		{"", 3},
		{"one row", 3},
		{strings.Repeat("line\n", 7) + "line", 10},
	}
	for _, tc := range cases {
		a.setInput([]rune(tc.input))
		if got := a.composerHeight(); got != tc.want {
			t.Errorf("input rows %d: composer height %d, want %d", len(tc.input), got, tc.want)
		}
		a.draw()
		for y := 20 - 1 - a.composerHeight(); y < 19; y++ {
			for x := 0; x < 80; x++ {
				_, _, style, _ := screen.GetContent(x, y)
				if _, bg, _ := style.Decompose(); bg != ui.BgTerm {
					t.Fatalf("composer cell %d,%d background %v", x, y, bg)
				}
			}
		}
	}
}

func TestComposerHeaderNarrowWidthDoesNotPanic(t *testing.T) {
	a, screen := visualFixture(t, 3, 8)
	a.st.Session.Title = strings.Repeat("界", 10)
	a.draw()
	// Also call the helper directly with the minimum supported width.
	a.drawComposerHeader(ui.Canvas{S: screen}, 0, 0, 3)
	if got := visualRow(screen, 0); got != "───" {
		t.Fatalf("narrow header %q", got)
	}
}
