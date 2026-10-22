package render

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// The active palette. SetTheme overwrites these; draw code reads them on every
// frame, so a theme change shows up on the next redraw.
var (
	BgTerm     tcell.Color
	BgUser     tcell.Color
	BgJSDone   tcell.Color
	BgJSRun    tcell.Color
	BgJSErr    tcell.Color
	BgPopup    tcell.Color
	BgPopupIn  tcell.Color
	BgPopupSel tcell.Color
	BgSelect   tcell.Color
	FgWhite    tcell.Color
	FgSoft     tcell.Color
	FgDim      tcell.Color
	FgFaint    tcell.Color
	FgBlack    tcell.Color
	Blue       tcell.Color
	Purple     tcell.Color
	Green      tcell.Color
	Yellow     tcell.Color
	Red        tcell.Color
	Orange     tcell.Color
)

func init() { SetTheme(DefaultTheme) }

type Style struct {
	FG, BG tcell.Color
	Bold   bool
}

func (s Style) TCell() tcell.Style {
	st := tcell.StyleDefault.Foreground(s.FG).Background(s.BG)
	if s.Bold {
		st = st.Bold(true)
	}
	return st
}

// Palette is the full set of colors the UI draws with.
type Palette struct {
	BgTerm, BgUser, BgJSDone, BgJSRun, BgJSErr, BgPopup, BgPopupIn, BgPopupSel, BgSelect tcell.Color
	FgWhite, FgSoft, FgDim, FgFaint, FgBlack                                             tcell.Color
	Blue, Purple, Green, Yellow, Red, Orange                                             tcell.Color
}

// Theme is a named palette.
type Theme struct {
	Name string
	Dark bool
	Palette
}

const DefaultTheme = "default"

func hex(s string) tcell.Color {
	var r, g, b int32
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
		panic("bad color " + s)
	}
	return tcell.NewRGBColor(r, g, b)
}

// Mix blends a toward b; t=0 gives a, t=1 gives b.
func Mix(a, b tcell.Color, t float64) tcell.Color { return mix(a, b, t) }

// mix blends a toward b; t=0 gives a, t=1 gives b.
func mix(a, b tcell.Color, t float64) tcell.Color {
	ar, ag, ab := a.RGB()
	br, bg, bb := b.RGB()
	f := func(x, y int32) int32 { return int32(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return tcell.NewRGBColor(f(ar, br), f(ag, bg), f(ab, bb))
}

// derive builds a full palette from the base colors of a scheme: surfaces are
// blended from the background toward the text color, highlights from the
// background toward the accents.
func derive(name string, dark bool, bg, fg, dim, blue, purple, green, yellow, red, orange string) Theme {
	b, f := hex(bg), hex(fg)
	popup := mix(b, f, 0.08)
	return Theme{Name: name, Dark: dark, Palette: Palette{
		BgTerm:     b,
		BgUser:     mix(b, f, 0.05),
		BgJSDone:   mix(b, hex(green), 0.10),
		BgJSRun:    mix(b, hex(green), 0.12),
		BgJSErr:    mix(b, hex(red), 0.10),
		BgPopup:    popup,
		BgPopupIn:  mix(popup, b, 0.5),
		BgPopupSel: mix(b, f, 0.14),
		BgSelect:   mix(b, hex(blue), 0.18),
		FgWhite:    f,
		FgSoft:     mix(f, b, 0.25),
		FgDim:      hex(dim),
		FgFaint:    mix(hex(dim), b, 0.45),
		FgBlack:    b,
		Blue:       hex(blue), Purple: hex(purple), Green: hex(green),
		Yellow: hex(yellow), Red: hex(red), Orange: hex(orange),
	}}
}

var themes = []Theme{
	derive(DefaultTheme, true, "000000", "EDEDED", "888888", "52A8FF", "BF7AF0", "62C073", "FFCA16", "FF6166", "F5A623"),
}

var currentTheme = DefaultTheme

// Themes lists the built-in themes in display order, then those of theme.json.
func Themes() []Theme { return append(append([]Theme(nil), themes...), extraThemes...) }

// CurrentTheme is the name of the active theme.
func CurrentTheme() string { return currentTheme }

// FindTheme looks a theme up by name.
func FindTheme(name string) (Theme, bool) {
	for _, t := range Themes() {
		if t.Name == name {
			return t, true
		}
	}
	return Theme{}, false
}

// SetTheme activates the named theme and reports whether it exists.
func SetTheme(name string) bool {
	t, ok := FindTheme(name)
	if !ok {
		return false
	}
	p := t.Palette
	BgTerm, BgUser, BgJSDone, BgJSRun, BgJSErr = p.BgTerm, p.BgUser, p.BgJSDone, p.BgJSRun, p.BgJSErr
	BgPopup, BgPopupIn, BgPopupSel, BgSelect = p.BgPopup, p.BgPopupIn, p.BgPopupSel, p.BgSelect
	FgWhite, FgSoft, FgDim, FgFaint, FgBlack = p.FgWhite, p.FgSoft, p.FgDim, p.FgFaint, p.FgBlack
	Blue, Purple, Green, Yellow, Red, Orange = p.Blue, p.Purple, p.Green, p.Yellow, p.Red, p.Orange
	currentTheme = name
	return true
}
