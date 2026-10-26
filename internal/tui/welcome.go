package tui

import (
	"os"
	"strconv"
	"strings"
	"time"

	"meno/internal/render"
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
	sep := seg{"   ·   ", render.FgDim, false}
	_, model, effort := a.selection()
	first := []seg{{"no model selected", render.FgDim, false}, {" · ", render.FgDim, false}, {"/model", render.Blue, false}}
	if a.needProvider() {
		first = []seg{{"no provider", render.FgDim, false}, {" · ", render.FgDim, false}, {"/login", render.Blue, false}}
	}
	if model != "" {
		first = []seg{{model, render.FgWhite, false}}
		if effort != "" {
			first = append(first, seg{" · " + effort, render.FgSoft, false})
		}
	}
	skills, servers, failed := 0, 0, 0
	for _, s := range a.st.Skills {
		if s.Enabled {
			skills++
		}
	}
	for _, s := range a.st.Servers {
		if s.Enabled {
			servers++
			if a.st.ServerInfo[s.Name].Err != "" {
				failed++
			}
		}
	}
	agents := []seg{{"○ ", render.FgDim, false}, {"no AGENTS.md", render.FgDim, false}}
	if a.agentsFound() {
		agents = []seg{{"● ", render.Green, false}, {"AGENTS.md", render.FgSoft, false}}
	}
	mcpGroup := []seg{{strconv.Itoa(servers) + " mcp", render.FgSoft, false}}
	if failed > 0 {
		mcpGroup = append(mcpGroup, seg{" · " + strconv.Itoa(failed) + " failed", render.Red, false})
	}
	groups := [][]seg{
		first,
		{{welcomePath(a.st.Cwd), render.FgSoft, false}},
		{{plural(skills, "skill", "skills"), render.FgSoft, false}},
		mcpGroup,
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
		if segsWidth(out) <= w-2*render.Gutter || n == 1 {
			return out
		}
	}
	return nil
}

func keyHints() []seg {
	gap := seg{"      ", render.FgDim, false}
	return []seg{
		{"/", render.Blue, false}, {" commands", render.FgDim, false}, gap,
		{"@", render.Purple, false}, {" files", render.FgDim, false}, gap,
		{"ctrl+v", render.Blue, false}, {" image", render.FgDim, false}, gap,
		{"/help", render.Purple, false}, {" all keys", render.FgDim, false},
	}
}

// drawWelcome draws the static prompt header and the info rows, all centered
// in the log area. The rest of the area stays blank.
func (a *App) drawWelcome(c render.Canvas, x, y, w, h int) {
	full := w >= 76 && h >= welcomeBlockH
	blockH := compactBlockH
	if full {
		blockH = welcomeBlockH
	}
	top := y + max(0, (h-blockH)/2)
	row := func(i int) (int, bool) { return top + i, top+i < y+h }

	tag := []seg{{tagline, render.FgSoft, false}}
	hints := keyHints()
	header := append([]seg{{"> ", render.Green, false}, {"meno", render.Blue, true}, {" ─ ", render.FgDim, false}}, a.versionSegs()...)
	if full {
		if r, ok := row(6); ok {
			drawSegs(c, x, r, w, header)
		}
		if r, ok := row(8); ok {
			drawSegs(c, x, r, w, tag)
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
		drawSegs(c, x, r, w, header)
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

// animTicker wakes only when an animated composer requested another frame.
func (a *App) animTicker() {
	t := time.NewTicker(logoInterval)
	defer t.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-t.C:
			if a.animOn.Load() {
				a.postEvent(animTick{})
			}
		}
	}
}
