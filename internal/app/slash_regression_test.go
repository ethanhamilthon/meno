package app

import (
	"strings"
	"testing"

	"meno/internal/ui"
)

func TestSkillNamespaceNeverExecutesApplicationCommand(t *testing.T) {
	a := refFixture(t)
	for _, input := range []string{"/skill:", "/skill: ", "/skill:unknown", "/skill:disabled", "/skill:api-security/file", "/api-security"} {
		a.setInput([]rune(input))
		if a.cmdActive() {
			t.Errorf("%q activated a command", input)
		}
		if input != "/skill:" && a.acActive() {
			t.Errorf("%q activated skill completion", input)
		}
	}
	a.setInput([]rune("/ski"))
	for i, row := range commandRows(a) {
		if row == "/skill:" {
			a.st.CmdSel = i
			a.st.CmdPicked = true
		}
	}
	a.onKey(enter())
	if string(a.st.Input) != "/skill:" || !a.acActive() || a.st.Over != OverNone {
		t.Fatal("skill handoff failed")
	}
	a.onKey(enter())
	if !strings.Contains(a.messageText(), "<skill>") || a.st.Running {
		t.Fatal("skill selection must not submit")
	}
}

func TestHistoryCrossBoundaryCopyReturnsInput(t *testing.T) {
	a := visualApp(t)
	a.setInput([]rune("unchanged draft"))
	a.st.Cursor = 2
	a.runCommand("edit")
	vimKeys(a, "kvjly")
	if a.st.Mode != ui.ModeInput || a.st.InLog || string(a.st.Input) != "unchanged draft" {
		t.Fatal("cross-boundary copy lost focus/draft")
	}
}

func TestEditNormalYankStaysInEdit(t *testing.T) {
	a := visualApp(t)
	a.runCommand("edit")
	vimKeys(a, "0yy")
	if a.st.Mode != ui.ModeEdit || a.st.Reg == "" {
		t.Fatal("normal yank changed mode")
	}
}

func TestComposerWavePreservesWideTextAndSelection(t *testing.T) {
	a, s := visualFixture(t, 80, 20)
	a.setInput([]rune("界 café\tend"))
	a.st.Cursor = 0
	a.runCommand("edit")
	vimKeys(a, "vl")
	a.draw()
	text := visualRow(s, a.st.ComposerY)
	a.st.Running = true
	a.draw()
	if visualRow(s, a.st.ComposerY) != text {
		t.Fatal("wide glyphs corrupted")
	}
	_, _, style, _ := s.GetContent(a.st.ComposerX, a.st.ComposerY)
	_, bg, _ := style.Decompose()
	if bg != ui.BgSelect {
		t.Fatal("wave overwrote selection")
	}
}
