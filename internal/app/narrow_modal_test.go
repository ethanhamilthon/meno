package app

import (
	"strings"
	"testing"

	"meno/internal/config"
	"meno/internal/ui"
	"meno/internal/version"
)

func TestNarrowSettingsChoices(t *testing.T) {
	for i, r := range settingRows {
		for _, choice := range config.Choices(r.key) {
			t.Run(r.key+"/"+choice, func(t *testing.T) {
				a, s := visualFixture(t, 40, 12)
				a.openOverlay(OverSettings)
				a.st.Sel = i
				if err := a.st.Settings.Set(r.key, choice); err != nil {
					t.Fatal(err)
				}
				a.draw()
				_, _, rowInTab, tab := settingTabFor(i)
				b := centered(40, 12, 66, 5+len(tab.rows))
				start := max(0, rowInTab-max(1, b.h-5)+1)
				y := b.y + 2 + rowInTab - start
				row := visualRow(s, y)
				if !strings.Contains(row, "▸ ") {
					t.Fatalf("selected label missing: %q", row)
				}
				for _, o := range []string{choice} {
					label := optLabel(&a.st.Settings, r.key, o)
					x := strings.Index(row, label)
					if x < 0 || ui.TextWidth(row[:x]) < b.x+4 || ui.TextWidth(row[:x])+ui.TextWidth(label) > b.x+b.w-2 {
						t.Fatalf("choice %q clips or collides: %q", label, row)
					}
					cellX := ui.TextWidth(row[:x])
					for col := cellX; col < cellX+ui.TextWidth(label); col++ {
						_, _, st, _ := s.GetContent(col, y)
						fg, bg, _ := st.Decompose()
						if o == choice && (fg != ui.FgBlack || bg == ui.BgPopup) {
							t.Fatalf("current choice %q not highlighted at %d", choice, col)
						}
						if o != choice && bg != ui.BgPopup {
							t.Fatalf("inactive choice %q highlighted", o)
						}
					}
				}
				if got := strings.TrimSpace(visualRow(s, b.y+b.h-1)); got == "" {
					t.Fatalf("hint clipped: %q", got)
				}
			})
		}
	}
}

func TestNarrowHelpHeaderAndCommands(t *testing.T) {
	a, s := visualFixture(t, 80, 12)
	a.openOverlay(OverHelp)
	a.draw()
	row := visualRow(s, centered(80, 12, 0, 12).y)
	if !strings.Contains(row, "help · meno") || !strings.Contains(row, version.Label()) {
		t.Fatalf("help header collides: %q", row)
	}
	for _, want := range []string{"fork from a message", "switch sessions"} {
		found := false
		for i, r := range helpRows {
			if r.desc != want {
				continue
			}
			found = true
			a.st.Sel = i
			a.draw()
			// The dialog is nearly full width here, so the key column is the wide one.
			pad := 16
			if len(r.key) > pad {
				pad = len(r.key)
			}
			if !strings.Contains(screenText(s), r.key+strings.Repeat(" ", pad-len(r.key))+want) {
				t.Fatalf("command description clipped: %q", want)
			}
		}
		if !found {
			t.Fatalf("missing corrected description %q", want)
		}
	}
}
