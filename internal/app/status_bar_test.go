package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func TestCountGlyphAndBadgeText(t *testing.T) {
	if countGlyph(3) != "3x◆" || countGlyph(12) != "12x◆" {
		t.Fatal("count glyph")
	}
	if got := badgeText("INSERT", nil); got != "INSERT" {
		t.Fatal(got)
	}
	if got := badgeText("NORMAL", []string{"subagents", "goal"}); got != "NORMAL #subagents #goal" {
		t.Fatal(got)
	}
}

func statusRow(t *testing.T, a *App, scr tcell.SimulationScreen, w int) string {
	t.Helper()
	scr.SetSize(w, 1)
	a.drawStatus(ui.Canvas{S: scr}, 0, 0, w)
	return strings.TrimRight(screenText(scr), " \n")
}

func TestStatusGroupsOfTheCurrentSession(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	if got := joinSegText(a.statusGroups()); got != "" {
		t.Fatalf("idle: %q", got)
	}
	a.st.Running = true
	if got := joinSegText(a.statusGroups()); got != " 1x◆" {
		t.Fatalf("running: %q", got)
	}
	a.onBgLoaded(bgLoaded{map[string]int{a.st.Session.ID: 3}})
	if got := joinSegText(a.statusGroups()); got != " 1x◆ 3x◆" {
		t.Fatalf("running with background work: %q", got)
	}
	a.st.Running = false
	if got := joinSegText(a.statusGroups()); got != " 3x◆" {
		t.Fatalf("background work only: %q", got)
	}
}

func TestStatusBarShowsModesInTheBadgeAndNotOnTheRight(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Session.Modes = []string{"subagents", "goal"}
	row := statusRow(t, a, scr, 120)
	if !strings.HasPrefix(row, " "+a.st.Mode.Label()+" #subagents #goal ") {
		t.Fatalf("badge: %q", row)
	}
	if strings.Contains(row, "/25") || strings.Count(row, "#goal") != 1 {
		t.Fatalf("modes must appear once, in the badge: %q", row)
	}
	a.st.Over = OverHelp
	if row := statusRow(t, a, scr, 120); !strings.HasPrefix(row, " HELP ") || strings.Contains(row, "#goal") {
		t.Fatalf("an overlay shows only its name: %q", row)
	}
}

func TestStatusBarDropsModesOnANarrowTerminal(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Session.Modes = []string{"subagents", "goal"}
	a.st.Running = true
	row := statusRow(t, a, scr, 48)
	if strings.Contains(row, "#") || !strings.Contains(row, "1x◆") {
		t.Fatalf("narrow: %q", row)
	}
	if !strings.HasSuffix(row, strings.TrimSpace(a.modelLabel())) {
		t.Fatalf("the model stays on the right: %q", row)
	}
}
