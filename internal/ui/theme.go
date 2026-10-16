package ui

import "github.com/gdamore/tcell/v2"

var (
	BgTerm     = tcell.NewRGBColor(0x0E, 0x10, 0x13)
	BgUser     = tcell.NewRGBColor(0x18, 0x1B, 0x20)
	BgJSDone   = tcell.NewRGBColor(0x12, 0x24, 0x1B)
	BgJSRun    = tcell.NewRGBColor(0x2E, 0x1C, 0x0E)
	BgPopup    = tcell.NewRGBColor(0x1C, 0x20, 0x26)
	BgPopupSel = tcell.NewRGBColor(0x28, 0x2E, 0x38)
	BgSelect   = tcell.NewRGBColor(0x26, 0x35, 0x4A)
	FgWhite    = tcell.NewRGBColor(0xF3, 0xF4, 0xF6)
	FgSoft     = tcell.NewRGBColor(0xB0, 0xB7, 0xC3)
	FgDim      = tcell.NewRGBColor(0x68, 0x70, 0x7E)
	FgBlack    = tcell.NewRGBColor(0x0E, 0x10, 0x13)
	Blue       = tcell.NewRGBColor(0x87, 0xAF, 0xFF)
	Purple     = tcell.NewRGBColor(0xD7, 0xAF, 0xFF)
	Green      = tcell.NewRGBColor(0x87, 0xD7, 0x87)
	Yellow     = tcell.NewRGBColor(0xFF, 0xD7, 0x87)
	Red        = tcell.NewRGBColor(0xFF, 0x87, 0x87)
	Orange     = tcell.NewRGBColor(0xFF, 0xAF, 0x87)
)

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
