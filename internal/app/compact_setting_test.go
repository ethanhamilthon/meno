package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
)

func TestSettingsDialogChangesAutoCompactPercentAndKeepsIt(t *testing.T) {
	a, scr, paths := e2eApp(t, &recClient{})
	defer scr.Fini()
	row := -1
	for i, r := range settingRows {
		if r.key == "compact_percent" {
			row = i
		}
	}
	if row < 0 {
		t.Fatal("no auto compact row in :settings")
	}
	a.openOverlay(OverSettings)
	a.st.Sel = row
	a.onKey(runeKey('-'))
	a.onKey(runeKey('-'))
	if a.st.Settings.CompactPercent != 70 {
		t.Fatalf("got %d", a.st.Settings.CompactPercent)
	}
	if got := optLabel(&a.st.Settings, "compact_percent", "percent"); got != "at 70%" {
		t.Fatal(got)
	}
	a.onKey(keyK(tcell.KeyEnter)) // cycling the single choice must not reset it
	s, err := config.NewStore(paths).LoadSettings()
	if err != nil || s.CompactPercent != 70 {
		t.Fatalf("not saved: %d %v", s.CompactPercent, err)
	}
}
