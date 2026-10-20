package app

import (
	"meno/internal/ui"
	"strings"
	"testing"
	"time"
)

func TestFooterCompactGeometryAndEscape(t *testing.T) {
	a, s := visualFixture(t, 80, 20)
	a.setInput([]rune("alpha beta"))
	cursor := a.st.Cursor
	vimKeys(a, "<esc>")
	if a.st.Mode != ui.ModeInput || a.st.Cursor != cursor || string(a.st.Input) != "alpha beta" {
		t.Fatal("Escape changed input")
	}
	a.draw()
	if a.composerHeight() != 2 {
		t.Fatalf("composer height %d", a.composerHeight())
	}
	footer := visualRow(s, 19)
	if strings.Contains(footer, "INPUT") || !strings.Contains(footer, a.ctxLabel()) {
		t.Fatalf("footer %q", footer)
	}
	if strings.Trim(string([]rune(footer)[78:]), "─ ") != "" {
		t.Fatalf("right gutter %q", footer)
	}
	if !strings.Contains(visualRow(s, 18), "alpha beta") {
		t.Fatal("blank row before footer")
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

func TestFooterWaveSharesHeaderPhase(t *testing.T) {
	a, s := visualFixture(t, 80, 20)
	a.st.Running = true
	base := time.Now()
	a.waveEpoch = base
	a.now = func() time.Time { return base.Add(1500 * time.Millisecond) }
	a.draw()
	header := headerFG(s, 20-a.bottomHeight(), 80)
	footer := headerFG(s, 19, 80)
	// Both rows begin with rule decoration on the same physical columns.
	if header[0] != footer[0] || header[1] != footer[1] {
		t.Fatal("header/footer wave phase differs")
	}
	first := footer
	a.now = func() time.Time { return base.Add(2500 * time.Millisecond) }
	a.draw()
	second := headerFG(s, 19, 80)
	same := true
	for i := range first {
		same = same && first[i] == second[i]
	}
	if same {
		t.Fatal("footer wave does not move")
	}
}
