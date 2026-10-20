package ui

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func luminance(c tcell.Color) float64 {
	r, g, b := c.RGB()
	f := func(v int32) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
}

func contrast(a, b tcell.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestThemeCatalog(t *testing.T) {
	list := Themes()
	if len(list) != 3 {
		t.Fatalf("%d themes, want 3", len(list))
	}
	seen := map[string]bool{}
	dark, light := 0, 0
	for _, th := range list {
		if seen[th.Name] {
			t.Fatalf("duplicate theme %s", th.Name)
		}
		seen[th.Name] = true
		if th.Dark {
			dark++
		} else {
			light++
		}
	}
	if !seen[DefaultTheme] || !seen["pure-black"] || !seen["catppuccin-mocha"] || dark != 3 || light != 0 {
		t.Fatalf("default %v, dark %d, light %d", seen[DefaultTheme], dark, light)
	}
}

func TestThemesAreReadable(t *testing.T) {
	for _, th := range Themes() {
		p := th.Palette
		bgs := map[string]tcell.Color{"term": p.BgTerm, "user": p.BgUser, "popup": p.BgPopup, "popup-sel": p.BgPopupSel, "js": p.BgJSDone, "js-run": p.BgJSRun, "select": p.BgSelect}
		for name, bg := range bgs {
			if c := contrast(p.FgWhite, bg); c < 4.5 {
				t.Errorf("%s: text on %s contrast %.1f", th.Name, name, c)
			}
		}
		check := func(what string, fg, bg tcell.Color, min float64) {
			if c := contrast(fg, bg); c < min {
				t.Errorf("%s: %s contrast %.1f < %.1f", th.Name, what, c, min)
			}
		}
		check("soft on term", p.FgSoft, p.BgTerm, 3.5)
		check("dim on term", p.FgDim, p.BgTerm, 2.5)
		check("dim on popup", p.FgDim, p.BgPopup, 2.0)
		for name, fg := range map[string]tcell.Color{"blue": p.Blue, "purple": p.Purple, "green": p.Green, "yellow": p.Yellow, "red": p.Red, "orange": p.Orange} {
			check(name+" on term", fg, p.BgTerm, 3.0)
			check(name+" on popup", fg, p.BgPopup, 2.5)
		}
		check("mode label on blue", p.FgBlack, p.Blue, 3.0)
		check("mode label on green", p.FgBlack, p.Green, 3.0)
		if th.Dark != (luminance(p.BgTerm) < 0.18) {
			t.Errorf("%s: Dark=%v does not match the background", th.Name, th.Dark)
		}
	}
}

func TestSetTheme(t *testing.T) {
	defer SetTheme(DefaultTheme)
	if SetTheme("no-such-theme") || CurrentTheme() != DefaultTheme {
		t.Fatal("unknown theme accepted")
	}
	def := BgTerm
	if !SetTheme("pure-black") || CurrentTheme() != "pure-black" || BgTerm == def || BgTerm != tcell.NewRGBColor(0, 0, 0) {
		t.Fatalf("pure-black not applied: %v", BgTerm)
	}
	if !SetTheme(DefaultTheme) || BgTerm != def || Blue != tcell.NewRGBColor(0x87, 0xAF, 0xFF) {
		t.Fatal("default not restored")
	}
}
