package tui

import "meno/internal/render"

// versionSegs is the version on the welcome screen, with a pointer to the
// newer release when the background check found one.
func (a *App) versionSegs() []seg {
	segs := []seg{{versionLabel, render.FgDim, false}}
	if a.st.UpdateTag != "" {
		segs = append(segs, seg{"  →  " + a.st.UpdateTag + " available · run: meno update", render.Yellow, false})
	}
	return segs
}
