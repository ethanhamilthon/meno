package tui

import (
	"meno/internal/config"
	"meno/internal/render"
	"os"
	"testing"
)

func TestLiveThemeKeepsLastValidPalette(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	defer render.ApplyPalette(render.DefaultPalette())
	c := config.DefaultThemeColors()
	c.Blue = "123456"
	a.onThemeLoaded(themeLoaded{colors: c})
	want := render.PaletteFromColors(c.Colors())
	if render.CurrentPalette() != want {
		t.Fatal("valid palette not applied")
	}
	if e := os.WriteFile(a.paths.ThemeFile(), []byte("{"), 0600); e != nil {
		t.Fatal(e)
	}
	_, err := config.ReadTheme(a.paths.ThemeFile())
	a.onThemeLoaded(themeLoaded{err: err})
	if render.CurrentPalette() != want {
		t.Fatal("partial edit replaced last valid palette")
	}
	n := len(a.st.Notices)
	a.onThemeLoaded(themeLoaded{err: err})
	if len(a.st.Notices) != n {
		t.Fatal("repeated warning")
	}
	a.idleTick = true
	a.onThemeLoaded(themeLoaded{colors: c})
	if !a.idleTick {
		t.Fatal("identical palette requested frame")
	}
	c.Red = "010203"
	a.onThemeLoaded(themeLoaded{colors: c})
	if a.idleTick {
		t.Fatal("changed palette did not request frame")
	}
	for _, r := range settingRows {
		if r.key == "theme" {
			t.Fatal("theme picker remains")
		}
	}
}
