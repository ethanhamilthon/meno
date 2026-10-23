package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"meno/internal/render"
)

func TestSlashCommandAndSkillNamespaces(t *testing.T) {
	a := refFixture(t)
	for input, want := range map[string]bool{"/": true, "/ne": true, "hello /mod": true, "/skills": true, "/skill:": false, "/skill:api": false, ":edit": false, "/tmp/project": false, "/model/file": false, "http://host/x": false, "a/b": false} {
		a.setInput([]rune(input))
		if got := a.cmdActive(); got != want {
			t.Errorf("%q command=%v want=%v", input, got, want)
		}
	}
	a.setInput([]rune("/skill:api"))
	if !a.acActive() {
		t.Fatal("skill completion missing")
	}
	a.onKey(keyK(tcell.KeyTab))
	if !strings.Contains(a.messageText(), "<skill>api-security</skill>") {
		t.Fatal(a.messageText())
	}
	a.setInput([]rune("/cd /model"))
	tok, ok := a.cmdToken()
	if !ok || tok.query != "cd /model" || !tok.args {
		t.Fatalf("nested path stole command: %+v", tok)
	}
}

func TestComposerPaddedBackgroundWave(t *testing.T) {
	a, s := visualFixture(t, 80, 20)
	a.st.Session.Title = "my session"
	a.setInput([]rune("text"))
	if a.composerHeight() != 3 {
		t.Fatalf("height %d", a.composerHeight())
	}
	a.draw()
	y := a.st.ComposerY
	for _, row := range []int{y - 1, y + 1} {
		if strings.TrimSpace(visualRow(s, row)) != "" {
			t.Fatal("padding not blank")
		}
	}
	_, _, style, _ := s.GetContent(2, y)
	fg, bg, _ := style.Decompose()
	if bg != render.BgPopup || fg != render.FgWhite {
		t.Fatal("wrong idle input style")
	}
	footer := visualRow(s, 19)
	if !strings.HasPrefix(strings.TrimSpace(footer), "my session") || strings.Contains(footer, "─") {
		t.Fatal(footer)
	}
	base := time.Now()
	a.waveEpoch = base
	a.st.Running = true
	a.now = func() time.Time { return base.Add(1500 * time.Millisecond) }
	a.draw()
	first := composerBackgrounds(s, y, 80)
	a.now = func() time.Time { return base.Add(2500 * time.Millisecond) }
	a.draw()
	second := composerBackgrounds(s, y, 80)
	changed := false
	for i := range first {
		changed = changed || first[i] != second[i]
	}
	if !changed {
		t.Fatal("background wave did not move")
	}
	if visualRow(s, 19) != footer {
		t.Fatal("footer changed during wave")
	}
	a.st.Running = false
	a.draw()
	for _, c := range composerBackgrounds(s, y, 80) {
		if c != render.BgPopup {
			t.Fatal("idle wave remained")
		}
	}
}

func composerBackgrounds(s tcell.SimulationScreen, y, w int) []tcell.Color {
	out := make([]tcell.Color, w)
	for x := range out {
		_, _, st, _ := s.GetContent(x, y)
		_, out[x], _ = st.Decompose()
	}
	return out
}
