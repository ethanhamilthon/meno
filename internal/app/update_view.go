package app

import "meno/internal/ui"

// versionSegs is the version on the welcome screen, with a pointer to the
// newer release when the background check found one.
func (a *App) versionSegs() []seg {
	segs := []seg{{versionLabel, ui.FgDim, false}}
	if a.st.UpdateTag != "" {
		segs = append(segs, seg{"  →  " + a.st.UpdateTag + " available · run: meno update", ui.Yellow, false})
	}
	return segs
}
