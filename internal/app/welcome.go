package app

import (
	"os"
	"strconv"
	"strings"
	"time"

	"meno/internal/ui"
)

// agentsFound reports whether a global or project AGENTS.md exists. The answer
// is cached for a couple of seconds because the welcome screen redraws often.
func (a *App) agentsFound() bool {
	if time.Since(a.agentsAt) < 2*time.Second {
		return a.agentsOK
	}
	p := a.paths
	p.Cwd = a.st.Cwd
	g, l := agentsPaths(p)
	a.agentsOK = false
	for _, f := range []string{l, g} {
		if st, err := os.Stat(f); err == nil && !st.IsDir() {
			a.agentsOK = true
			break
		}
	}
	a.agentsAt = time.Now()
	return a.agentsOK
}

func welcomePath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && (p == home || strings.HasPrefix(p, home+string(os.PathSeparator))) {
		p = "~" + p[len(home):]
	}
	if r := []rune(p); len(r) > 36 {
		p = "…" + string(r[len(r)-35:])
	}
	return p
}

func plural(n int, one, many string) string {
	if n == 1 {
		return strconv.Itoa(n) + " " + one
	}
	return strconv.Itoa(n) + " " + many
}

// infoLine is the model · effort · cwd · skills · mcp · AGENTS.md row. Groups
// are dropped from the end when the terminal is too narrow.
func (a *App) infoLine(w int) []seg {
	sep := seg{"   ·   ", ui.FgDim, false}
	_, model, effort := a.selection()
	first := []seg{{"no model selected", ui.FgDim, false}, {" · ", ui.FgDim, false}, {":model", ui.Blue, false}}
	if model != "" {
		first = []seg{{model, ui.FgWhite, false}}
		if effort != "" {
			first = append(first, seg{" · " + effort, ui.FgSoft, false})
		}
	}
	skills, servers := 0, 0
	for _, s := range a.st.Skills {
		if s.Enabled {
			skills++
		}
	}
	for _, s := range a.st.Servers {
		if s.Enabled {
			servers++
		}
	}
	agents := []seg{{"○ ", ui.FgDim, false}, {"no AGENTS.md", ui.FgDim, false}}
	if a.agentsFound() {
		agents = []seg{{"● ", ui.Green, false}, {"AGENTS.md", ui.FgSoft, false}}
	}
	groups := [][]seg{
		first,
		{{welcomePath(a.st.Cwd), ui.FgSoft, false}},
		{{plural(skills, "skill", "skills"), ui.FgSoft, false}},
		{{strconv.Itoa(servers) + " mcp", ui.FgSoft, false}},
		agents,
	}
	for n := len(groups); n >= 1; n-- {
		var out []seg
		for i, g := range groups[:n] {
			if i > 0 {
				out = append(out, sep)
			}
			out = append(out, g...)
		}
		if segsWidth(out) <= w-2 || n == 1 {
			return out
		}
	}
	return nil
}

func keyHints() []seg {
	gap := seg{"      ", ui.FgDim, false}
	return []seg{
		{"i", ui.Blue, false}, {" write", ui.FgDim, false}, gap,
		{"/", ui.Purple, false}, {" skills", ui.FgDim, false}, gap,
		{":", ui.Blue, false}, {" commands", ui.FgDim, false}, gap,
		{":help", ui.Purple, false}, {" all keys", ui.FgDim, false},
	}
}

// drawWelcome draws the animated wave, the MENO wordmark and the info rows,
// all centered in the log area.
func (a *App) drawWelcome(c ui.Canvas, x, y, w, h int) {
	a.waveOn.Store(true)
	t := float64(time.Now().UnixMilli()) / 1000
	full := w >= logoW+10 && h >= welcomeBlockH
	blockH := compactBlockH
	if full {
		blockH = welcomeBlockH
	}
	top := y + max(0, (h-blockH)/2)
	row := func(i int) (int, bool) { return top + i, top+i < y+h }

	logoX, logoY := x+(w-logoW)/2, top+3
	if full {
		drawWave(c, x, y, w, h, t, logoX-4-x, logoY-1-y, logoW+9, logoH+3)
	} else {
		drawWave(c, x, y, w, h, t, 0, 0, 0, 0)
	}

	tag := []seg{{tagline, ui.FgSoft, false}}
	hints := keyHints()
	if full {
		if r, ok := row(0); ok {
			drawSegs(c, x, r, w, tag)
		}
		drawLogo(c, logoX, logoY, x, y, w, h, t)
		if r, ok := row(12); ok {
			drawSegs(c, x, r, w, a.versionSegs())
		}
		if r, ok := row(15); ok {
			drawSegs(c, x, r, w, a.infoLine(w))
		}
		if r, ok := row(18); ok {
			drawSegs(c, x, r, w, hints)
		}
		return
	}
	if r, ok := row(0); ok {
		drawSegs(c, x, r, w, append([]seg{{"meno", ui.Blue, true}, {" ", ui.FgDim, false}}, a.versionSegs()...))
	}
	if r, ok := row(1); ok {
		drawSegs(c, x, r, w, tag)
	}
	if r, ok := row(3); ok {
		drawSegs(c, x, r, w, a.infoLine(w))
	}
	if r, ok := row(4); ok {
		drawSegs(c, x, r, w, hints)
	}
}

// waveTicker wakes the render loop while the welcome screen is on show.
func (a *App) waveTicker() {
	t := time.NewTicker(waveInterval)
	defer t.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-t.C:
			if a.waveOn.Load() {
				a.postEvent(waveTick{})
			}
		}
	}
}
