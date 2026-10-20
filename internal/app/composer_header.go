package app

import (
	"os"
	"path/filepath"
	"strings"

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

// drawComposerHeader draws the session name at the right end of the rule.
func (a *App) drawComposerHeader(c ui.Canvas, x, y, w int) {
	name := ""
	if w >= 4 {
		name = ui.Truncate(a.composerHeaderLabel(), min(max(1, w*composerHeaderMax/100), w-2))
	}
	st := ui.Style{FG: ui.FgDim, BG: ui.BgTerm}.TCell()
	if name == "" {
		c.Text(x, y, w, strings.Repeat("─", w), st)
		return
	}
	n := ui.TextWidth(name)
	line := max(0, w-n-1)
	c.Text(x, y, line, strings.Repeat("─", line), st)
	c.Text(x+w-n, y, n, name, st)
}
