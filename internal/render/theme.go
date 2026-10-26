package render

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// The active palette. ApplyPalette overwrites these; draw code reads them on every
// frame, so a theme change shows up on the next redraw.
var (
	BgTerm     tcell.Color
	BgUser     tcell.Color
	BgDiffAdd  tcell.Color
	BgDiffDel  tcell.Color
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

func init() { ApplyPalette(DefaultPalette()) }

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
	BgTerm, BgUser, BgDiffAdd, BgDiffDel, BgPopup, BgPopupIn, BgPopupSel, BgSelect tcell.Color
	FgWhite, FgSoft, FgDim, FgFaint, FgBlack                                       tcell.Color
	Blue, Purple, Green, Yellow, Red, Orange                                       tcell.Color
}

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
func derive(bg, fg, dim, blue, purple, green, yellow, red, orange string) Palette {
	b, f := hex(bg), hex(fg)
	popup := mix(b, f, 0.08)
	return Palette{
		BgTerm:     b,
		BgUser:     mix(b, f, 0.05),
		BgDiffAdd:  mix(b, hex(green), 0.10),
		BgDiffDel:  mix(b, hex(red), 0.10),
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
	}
}

// PaletteFromColors derives terminal surfaces from validated base colors.
func PaletteFromColors(c [9]string) Palette {
	return derive(c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8])
}

// DefaultPalette is the embedded fallback; config owns the on-disk defaults.
// TestDefaultMatchesFilePalette prevents these layer-independent values drifting.
func DefaultPalette() Palette {
	return PaletteFromColors([9]string{"17191C", "DBDFE5", "888F99", "8CB8FF", "B7A5DE", "9DCC9D", "E1CD8C", "E39A9A", "E9BC86"})
}

// ApplyPalette is called on the UI goroutine after validation.
func ApplyPalette(p Palette) {
	BgTerm, BgUser, BgDiffAdd, BgDiffDel = p.BgTerm, p.BgUser, p.BgDiffAdd, p.BgDiffDel
	BgPopup, BgPopupIn, BgPopupSel, BgSelect = p.BgPopup, p.BgPopupIn, p.BgPopupSel, p.BgSelect
	FgWhite, FgSoft, FgDim, FgFaint, FgBlack = p.FgWhite, p.FgSoft, p.FgDim, p.FgFaint, p.FgBlack
	Blue, Purple, Green, Yellow, Red, Orange = p.Blue, p.Purple, p.Green, p.Yellow, p.Red, p.Orange
}

// CurrentPalette is comparable, so layout keys cover every semantic color.
func CurrentPalette() Palette {
	return Palette{BgTerm, BgUser, BgDiffAdd, BgDiffDel, BgPopup, BgPopupIn, BgPopupSel, BgSelect, FgWhite, FgSoft, FgDim, FgFaint, FgBlack, Blue, Purple, Green, Yellow, Red, Orange}
}
