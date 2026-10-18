package ui

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

func Base() tcell.Style { return Style{FG: FgWhite, BG: BgTerm}.TCell() }

// Palette is the full set of colors the UI draws with.
type Palette struct {
	BgTerm, BgUser, BgJSDone, BgJSRun, BgJSErr, BgPopup, BgPopupIn, BgPopupSel, BgSelect tcell.Color
	FgWhite, FgSoft, FgDim, FgBlack                                                      tcell.Color
	Blue, Purple, Green, Yellow, Red, Orange                                             tcell.Color
}

// Theme is a named palette.
type Theme struct {
	Name string
	Dark bool
	Palette
}

const DefaultTheme = "meno"

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
		BgJSRun:    mix(b, hex(orange), 0.12),
		BgJSErr:    mix(b, hex(red), 0.10),
		BgPopup:    popup,
		BgPopupIn:  mix(popup, b, 0.5),
		BgPopupSel: mix(b, f, 0.14),
		BgSelect:   mix(b, hex(blue), 0.18),
		FgWhite:    f,
		FgSoft:     mix(f, b, 0.25),
		FgDim:      hex(dim),
		FgBlack:    b,
		Blue:       hex(blue), Purple: hex(purple), Green: hex(green),
		Yellow: hex(yellow), Red: hex(red), Orange: hex(orange),
	}}
}

var themes = []Theme{
	{Name: DefaultTheme, Dark: true, Palette: Palette{
		BgTerm: hex("0E1013"), BgUser: hex("181B20"), BgJSDone: hex("12241B"), BgJSRun: hex("2E1C0E"), BgJSErr: hex("2B1E21"),
		BgPopup: hex("1C2026"), BgPopupIn: hex("15181D"), BgPopupSel: hex("282E38"), BgSelect: hex("26354A"),
		FgWhite: hex("F3F4F6"), FgSoft: hex("B0B7C3"), FgDim: hex("68707E"), FgBlack: hex("0E1013"),
		Blue: hex("87AFFF"), Purple: hex("D7AFFF"), Green: hex("87D787"), Yellow: hex("FFD787"), Red: hex("FF8787"), Orange: hex("FFAF87"),
	}},
	derive("meno-light", false, "F6F7F9", "1C2026", "7A828E", "3B6FD4", "8B5CC8", "2F8F4E", "B07A10", "CC4646", "C2662B"),
	derive("dracula", true, "282A36", "F8F8F2", "6272A4", "8BE9FD", "BD93F9", "50FA7B", "F1FA8C", "FF5555", "FFB86C"),
	derive("nord", true, "2E3440", "ECEFF4", "6C7A96", "88C0D0", "B48EAD", "A3BE8C", "EBCB8B", "D8737C", "D08770"),
	derive("gruvbox-dark", true, "282828", "EBDBB2", "928374", "83A598", "D3869B", "B8BB26", "FABD2F", "FB4934", "FE8019"),
	derive("gruvbox-light", false, "FBF1C7", "3C3836", "928374", "076678", "8F3F71", "79740E", "B57614", "9D0006", "AF3A03"),
	derive("solarized-dark", true, "002B36", "9FADAD", "5F7780", "268BD2", "6C71C4", "859900", "B58900", "DC322F", "CB4B16"),
	derive("solarized-light", false, "FDF6E3", "475B62", "7A8B8F", "268BD2", "6C71C4", "6F8000", "946F00", "DC322F", "CB4B16"),
	derive("tokyo-night", true, "1A1B26", "C0CAF5", "565F89", "7AA2F7", "BB9AF7", "9ECE6A", "E0AF68", "F7768E", "FF9E64"),
	derive("catppuccin-mocha", true, "1E1E2E", "CDD6F4", "6C7086", "89B4FA", "CBA6F7", "A6E3A1", "F9E2AF", "F38BA8", "FAB387"),
	derive("catppuccin-latte", false, "EFF1F5", "4C4F69", "7C7F93", "1E66F5", "8839EF", "358A24", "A8680F", "D20F39", "D2530A"),
	derive("one-dark", true, "282C34", "ABB2BF", "7F8798", "61AFEF", "C678DD", "98C379", "E5C07B", "E06C75", "D19A66"),
	derive("one-light", false, "FAFAFA", "383A42", "8A8B92", "4078F2", "A626A4", "50A14F", "C18401", "E45649", "986801"),
	derive("monokai", true, "272822", "F8F8F2", "75715E", "66D9EF", "AE81FF", "A6E22E", "E6DB74", "F92672", "FD971F"),
	derive("rose-pine", true, "191724", "E0DEF4", "6E6A86", "9CCFD8", "C4A7E7", "9ECE9A", "F6C177", "EB6F92", "EA9A97"),
	derive("everforest", true, "2D353B", "D3C6AA", "859289", "7FBBB3", "D699B6", "A7C080", "DBBC7F", "E67E80", "E69875"),
	derive("kanagawa", true, "1F1F28", "DCD7BA", "727169", "7E9CD8", "957FB8", "98BB6C", "E6C384", "E46876", "FFA066"),
	derive("ayu-dark", true, "0B0E14", "BFBDB6", "565B66", "59C2FF", "D2A6FF", "AAD94C", "E6B450", "F07178", "FF8F40"),
	derive("github-dark", true, "0D1117", "E6EDF3", "7D8590", "79C0FF", "D2A8FF", "7EE787", "E3B341", "FF7B72", "FFA657"),
	derive("github-light", false, "FFFFFF", "1F2328", "656D76", "0969DA", "8250DF", "1A7F37", "9A6700", "CF222E", "BC4C00"),
}

var currentTheme = DefaultTheme

// Themes lists the built-in themes in display order.
func Themes() []Theme { return append([]Theme(nil), themes...) }

// CurrentTheme is the name of the active theme.
func CurrentTheme() string { return currentTheme }

// FindTheme looks a theme up by name.
func FindTheme(name string) (Theme, bool) {
	for _, t := range themes {
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
	FgWhite, FgSoft, FgDim, FgBlack = p.FgWhite, p.FgSoft, p.FgDim, p.FgBlack
	Blue, Purple, Green, Yellow, Red, Orange = p.Blue, p.Purple, p.Green, p.Yellow, p.Red, p.Orange
	currentTheme = name
	return true
}
