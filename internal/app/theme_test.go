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

func TestThemeCommandListsAllThemesAndPreviews(t *testing.T) {
	a, s := themeApp(t)
	a.runCommand("theme")
	if a.st.Over != OverTheme {
		t.Fatal("theme dialog did not open")
	}
	a.draw()
	text := screenText(s)
	if !strings.Contains(text, "1 / 20") && !strings.Contains(text, "20 / 20") {
		t.Fatalf("counter missing:\n%s", text)
	}
	if !strings.Contains(text, "meno") || !strings.Contains(text, "dracula") {
		t.Fatalf("themes missing:\n%s", text)
	}
	// Moving the selection previews the theme at once.
	a.onKey(keyK(tcell.KeyDown))
	a.onKey(keyK(tcell.KeyDown))
	want := a.themeIdx()[2].Name
	if ui.CurrentTheme() != want {
		t.Fatalf("preview %q, want %q", ui.CurrentTheme(), want)
	}
	a.draw()
	preview, _ := ui.FindTheme(want)
	if ui.BgPopup != preview.BgPopup {
		t.Fatal("popup colors do not follow the preview")
	}
	sw, sh := s.Size()
	b := centered(sw, sh, 52, 6+len(a.themeIdx()))
	_, _, st, _ := s.GetContent(b.x+1, b.y+1)
	if _, bg, _ := st.Decompose(); bg != preview.BgPopup {
		t.Fatalf("dialog drawn with %v, want the preview's %v", bg, preview.BgPopup)
	}
	// Esc throws the preview away.
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Over != OverNone || ui.CurrentTheme() != "meno" || a.st.Settings.Theme != "meno" {
		t.Fatalf("esc: over %v theme %q saved %q", a.st.Over, ui.CurrentTheme(), a.st.Settings.Theme)
	}
}

func TestThemeEnterAppliesAndPersists(t *testing.T) {
	a, s := themeApp(t)
	a.runCommand("theme")
	typeRunes(a, "dracula")
	if list := a.themeIdx(); len(list) != 1 || ui.CurrentTheme() != "dracula" {
		t.Fatalf("filter %v, preview %q", list, ui.CurrentTheme())
	}
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Over != OverNone || a.st.Settings.Theme != "dracula" || ui.CurrentTheme() != "dracula" {
		t.Fatalf("enter: over %v saved %q current %q", a.st.Over, a.st.Settings.Theme, ui.CurrentTheme())
	}
	saved, err := a.store.LoadSettings()
	if err != nil || saved.Theme != "dracula" {
		t.Fatalf("not persisted: %+v %v", saved, err)
	}
	a.draw()
	dracula, _ := ui.FindTheme("dracula")
	// The whole timeline area uses the theme background.
	_, _, st, _ := s.GetContent(0, 0)
	if _, bg, _ := st.Decompose(); bg != dracula.BgTerm {
		t.Fatalf("background %v, want %v", bg, dracula.BgTerm)
	}
	// The welcome screen's wordmark uses the theme accent, not the startup one.
	found := false
	w, h := s.Size()
	for y := 0; y < h && !found; y++ {
		for x := 0; x < w; x++ {
			r, _, st, _ := s.GetContent(x, y)
			if fg, _, _ := st.Decompose(); r == '█' && fg == dracula.Blue {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("wordmark not recolored")
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

func TestThemeDialogFitsSmallTerminal(t *testing.T) {
	a, s := themeApp(t)
	s.SetSize(60, 12)
	a.runCommand("theme")
	for i := 0; i < 19; i++ {
		a.onKey(keyK(tcell.KeyDown))
	}
	a.draw()
	if text := screenText(s); !strings.Contains(text, a.themeIdx()[19].Name) {
		t.Fatalf("selected theme scrolled out of view:\n%s", text)
	}
}
