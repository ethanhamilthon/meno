package app

import (
	"os"
	"path/filepath"
	"time"

	"github.com/rivo/uniseg"

	"meno/internal/ui"
)

// composerHeaderMax is the share of the width the name may take.
const composerHeaderMax = 30 // percent

// composerHeaderLabel is the session title, or its folder while no title exists.
func (a *App) composerHeaderLabel() string {
	if a.st.Session != nil {
		if title := a.st.Session.DisplayTitle(); title != "" && title != "untitled" {
			return oneLine(title)
		}
	}
	cwd := a.st.Cwd
	if home, err := os.UserHomeDir(); err == nil && cwd == home {
		return "~"
	}
	if base := filepath.Base(cwd); base != "" && base != "." && base != "/" {
		return base
	}
	return "~"
}

// drawComposerHeader draws the session name at the right end of the rule. While
// an agent works, a green wave runs through the rule and the name.
func (a *App) drawComposerHeader(c ui.Canvas, x, y, w int) {
	name := ""
	if w >= 2*ui.Gutter+4 {
		name = ui.Truncate(a.composerHeaderLabel(), min(max(1, w*composerHeaderMax/100), w-2*ui.Gutter-3))
	}
	n := ui.TextWidth(name)
	cells := make([]string, w) // "" is a space
	line := w
	if name != "" {
		line = max(0, w-ui.Gutter-n-1)
		for i := w - ui.Gutter; i < w; i++ {
			cells[i] = "─" // the rule may run through the right gutter; the name may not
		}
	}
	for i := 0; i < line; i++ {
		cells[i] = "─"
	}
	if name != "" {
		col := w - ui.Gutter - n
		g := uniseg.NewGraphemes(name)
		for g.Next() {
			cells[col] = g.Str()
			gw := ui.TextWidth(g.Str())
			col += max(1, gw)
			if col >= w-ui.Gutter {
				break
			}
		}
	}
	textColors := a.chromeColors(w, chromeText())
	cols := a.chromeColors(w, ui.FgFaint)
	for i, s := range cells {
		if s == "" {
			continue
		}
		fg := ui.FgFaint
		if s != "─" {
			fg = chromeText()
		}
		if cols != nil {
			if s == "─" {
				fg = cols[i]
			} else {
				fg = textColors[i]
			}
		}
		c.Text(x+i, y, max(1, ui.TextWidth(s)), s, ui.Style{FG: fg, BG: ui.BgTerm}.TCell())
	}
}

// clock is the time source of animations.
func (a *App) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}
