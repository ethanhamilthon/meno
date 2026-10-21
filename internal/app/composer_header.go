package app

import (
	"os"
	"path/filepath"
	"time"
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

// clock is the time source of animations.
func (a *App) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}
