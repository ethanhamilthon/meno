package tui

import (
	"meno/internal/render"
	"strings"
	"testing"
)

func TestFooterCompactGeometryAndEscape(t *testing.T) {
	a, s := visualFixture(t, 80, 20)
	a.setInput([]rune("alpha beta"))
	cursor := a.st.Cursor
	typeKeys(a, "<esc>")
	if a.st.Mode != render.ModeInput || a.st.Cursor != cursor || string(a.st.Input) != "alpha beta" {
		t.Fatal("Escape changed input")
	}
	a.draw()
	if a.composerHeight() != 3 {
		t.Fatalf("composer height %d", a.composerHeight())
	}
	footer := visualRow(s, 19)
	if strings.Contains(footer, "INPUT") || !strings.Contains(footer, a.ctxLabel()) {
		t.Fatalf("footer %q", footer)
	}
	if strings.Trim(string([]rune(footer)[78:]), "─ ") != "" {
		t.Fatalf("right gutter %q", footer)
	}
	if !strings.Contains(visualRow(s, 17), "alpha beta") {
		t.Fatal("input missing above bottom padding")
	}
}

func TestFooterReservesContextAtEveryWidth(t *testing.T) {
	for _, width := range []int{1, 3, 20, 80, 120} {
		a, s := visualFixture(t, width, 15)
		a.draw()
		if width >= 20 && !strings.Contains(visualRow(s, 14), a.ctxLabel()) {
			t.Fatalf("width %d lost context", width)
		}
	}
}
