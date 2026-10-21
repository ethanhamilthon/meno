package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"meno/internal/ui"
)

func TestVisualHistoryYankReturnsComposerFocus(t *testing.T) {
	for _, draft := range []string{"", "keep this draft"} {
		for _, keys := range []string{"vly", "vY", "<c-v>ly"} {
			t.Run(draft+keys, func(t *testing.T) {
				a := visualApp(t)
				a.setInput([]rune(draft))
				a.st.Cursor = min(3, len(a.st.Input))
				cursor := a.st.Cursor
				a.runCommand("edit")
				if !a.st.InLog {
					vimKeys(a, "k")
				}
				for i, line := range a.logText() {
					if strings.Contains(string(line), "hello world") {
						a.st.InLog = true
						a.st.LogLine = i
						a.st.LogCol = 0
						break
					}
				}
				// Ctrl+V is delivered explicitly by the simulation helper below.
				if strings.HasPrefix(keys, "<c-v>") {
					a.onKey(keyK(tcell.KeyCtrlV))
					vimKeys(a, "ly")
				} else {
					vimKeys(a, keys)
				}
				if a.st.Mode != ui.ModeInput || a.st.InLog || string(a.st.Input) != draft || a.st.Cursor != cursor {
					t.Fatalf("mode=%v history=%v draft=%q cursor=%d want=%d", a.st.Mode, a.st.InLog, string(a.st.Input), a.st.Cursor, cursor)
				}
				if a.st.Reg == "" {
					t.Fatal("empty register")
				}
				a.draw()
				x, y, visible := a.scr.(tcell.SimulationScreen).GetCursor()
				if !visible || x != a.cur.x || y != a.cur.y || y < a.st.ComposerY {
					t.Fatalf("cursor %d,%d visible=%v", x, y, visible)
				}
			})
		}
	}
}

func TestSlashCommandAndSkillNamespaces(t *testing.T) {
	a := refFixture(t)
	for input, want := range map[string]bool{"/": true, "/ed": true, "hello /mod": true, "/skills": true, "/skill:": false, "/skill:api": false, ":edit": false, "/tmp/project": false, "/model/file": false, "http://host/x": false, "a/b": false} {
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
	if bg != ui.BgPopup || fg != ui.FgWhite {
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
		if c != ui.BgPopup {
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
