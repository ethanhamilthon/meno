package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/ui"
)

func themeApp(t *testing.T) (*App, tcell.SimulationScreen) {
	t.Helper()
	a, s := newTestApp(t)
	t.Cleanup(func() { ui.SetTheme(ui.DefaultTheme); s.Fini() })
	return a, s
}

func TestSettingsThemeCyclesAndPersists(t *testing.T) {
	a, s := themeApp(t)
	a.runCommand("settings")
	if a.st.Over != OverSettings {
		t.Fatal("settings dialog did not open")
	}
	a.draw()
	text := screenText(s)
	if !strings.Contains(text, "General") || !strings.Contains(text, "theme") || !strings.Contains(text, "meno") {
		t.Fatalf("General tab or theme row missing:\n%s", text)
	}
	a.onKey(keyK(tcell.KeyRight))
	a.onKey(keyK(tcell.KeyRight))
	want := ui.Themes()[2].Name
	if ui.CurrentTheme() != want || a.st.Settings.Theme != want {
		t.Fatalf("theme %q/%q, want %q", ui.CurrentTheme(), a.st.Settings.Theme, want)
	}
	saved, err := a.store.LoadSettings()
	if err != nil || saved.Theme != want {
		t.Fatalf("not persisted: %+v %v", saved, err)
	}
	a.draw()
	preview, _ := ui.FindTheme(want)
	if ui.BgPopup != preview.BgPopup {
		t.Fatal("popup colors do not follow the selected theme")
	}
	a.onKey(keyK(tcell.KeyEsc))
	a.draw()
	_, _, st, _ := s.GetContent(0, 0)
	if _, bg, _ := st.Decompose(); bg != preview.BgTerm {
		t.Fatalf("background %v, want %v", bg, preview.BgTerm)
	}
}

func TestThemeLoadedAtStartupWithFallback(t *testing.T) {
	dir := t.TempDir()
	st := config.NewStore(config.Paths{Home: dir, Cwd: dir})
	set := config.DefaultSettings()
	set.Theme = "nord"
	if err := st.SaveSettings(set); err != nil {
		t.Fatal(err)
	}
	defer ui.SetTheme(ui.DefaultTheme)
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	if _, err := New(scr, config.Paths{Home: dir, Cwd: dir}, nil); err != nil || ui.CurrentTheme() != "nord" {
		t.Fatalf("startup theme %q err %v", ui.CurrentTheme(), err)
	}
	set.Theme = "no-such-theme"
	if err := st.SaveSettings(set); err != nil {
		t.Fatal(err)
	}
	if _, err := New(scr, config.Paths{Home: dir, Cwd: dir}, nil); err != nil || ui.CurrentTheme() != ui.DefaultTheme {
		t.Fatalf("fallback %q err %v", ui.CurrentTheme(), err)
	}
}
