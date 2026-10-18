package app

import (
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
	"meno/internal/version"
)

// logoRows is the MENO wordmark: four 5x7 pixel letters, each pixel three
// glyphs wide, filled with "hacker" symbols. Letters are 15 cells wide with
// two cells between them.
var logoRows = [...]string{
	`|&%         $0=  $]0##%0$$#%1{<%  !;[         }$~     $1*}?~%+%   `,
	`1;+**#   ^0^+}&  =[|              /^[!]/      [0:  #~+         $#~`,
	`1#<   :]%   {/1  0?$              ;0+   1~:   %=|  =&{         1*:`,
	`^<{   *;|   ]~&  }\\>#;{>\{~&     :?%   \]~   =;;  %]!         =]=`,
	`$%<         ]/:  ~<=              ]!&      &=/+++  {^1         ]&>`,
	`<0=         1~&  <&#              <\:         $1?  +/&         &!?`,
	`/>}         ]1}  +?^*#?|0}};[1#[  1\*         ?!%     *]/%^>![%   `,
}

const (
	logoW         = 66 // cells
	logoH         = 7  // rows, without the shadow
	logoLetterW   = 15
	logoLetterGap = 2
	welcomeBlockH = 19 // tagline, wordmark, version, info line and hints
	compactBlockH = 5
	waveRamp      = " .:-=+*#"
	tagline       = "efficient and stable AI harness"
)

// versionLabel is shown on the welcome screen and in the help title.
var versionLabel = version.Label()

// waveTick wakes the render loop while the welcome screen animates.
type waveTick struct{}

// waveInterval is the frame time of the welcome background.
const waveInterval = 100 * time.Millisecond

type seg struct {
	text string
	fg   tcell.Color
	bold bool
}

func segsWidth(ss []seg) int {
	n := 0
	for _, s := range ss {
		n += ui.TextWidth(s.text)
	}
	return n
}

// drawSegs draws the segments as one line centered in [x, x+w).
func drawSegs(c ui.Canvas, x, y, w int, ss []seg) {
	col := x + max(0, (w-segsWidth(ss))/2)
	for _, s := range ss {
		st := ui.Style{FG: s.fg, BG: ui.BgTerm, Bold: s.bold}.TCell()
		col += c.Text(col, y, max(0, x+w-col), s.text, st)
	}
}

// waveGrain is a fixed pseudo-random value in [0,1) for a cell.
func waveGrain(col, row int) float64 {
	h := uint32(col)*374761393 + uint32(row)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	return float64((h^(h>>16))&0xffff) / 65536
}

// drawWave paints the faint ASCII noise wave behind the welcome screen. The
// cells inside the hole (the wordmark and its shadow) stay empty.
func drawWave(c ui.Canvas, x, y, w, h int, t float64, hx, hy, hw, hh int) {
	for r := 0; r < h; r++ {
		fr := float64(r)
		rowT := (math.Sin(fr*0.4-t*0.6) + 1) / 2
		st := tcell.StyleDefault.Foreground(ui.Mix(ui.BgTerm, ui.Blue, 0.05+0.10*rowT)).Background(ui.BgTerm)
		for col := 0; col < w; col++ {
			if col >= hx && col < hx+hw && r >= hy && r < hy+hh {
				continue
			}
			fc := float64(col)
			v := (math.Sin(fc*0.11+fr*0.35+math.Sin(fr*0.2)*2-t*0.9) +
				0.7*math.Sin(fc*0.05-fr*0.5+t*0.5) +
				0.5*math.Sin((fc+fr*3)*0.07-t*0.7) + 2.2) / 4.4
			k := int(v*v*8 + waveGrain(col, r)*0.8)
			k = min(max(k, 0), len(waveRamp)-1)
			if k == 0 {
				continue
			}
			c.S.SetContent(x+col, y+r, rune(waveRamp[k]), nil, st)
		}
	}
}

// drawLogo draws the wordmark at (lx, ly): a dim shadow one cell down and to
// the right, then the letters in a blue to purple gradient.
func drawLogo(c ui.Canvas, lx, ly, x, y, w, h int) {
	put := func(cx, cy int, r rune, st tcell.Style) {
		if cx >= x && cx < x+w && cy >= y && cy < y+h {
			c.S.SetContent(cx, cy, r, nil, st)
		}
	}
	shadow := ui.Style{FG: ui.Mix(ui.BgTerm, ui.Blue, 0.12), BG: ui.BgTerm, Bold: true}.TCell()
	for ri, row := range logoRows {
		for i, r := range []rune(row) {
			if r != ' ' {
				put(lx+i+1, ly+ri+1, r, shadow)
			}
		}
	}
	for ri, row := range logoRows {
		for i, r := range []rune(row) {
			if r == ' ' {
				continue
			}
			letter := i / (logoLetterW + logoLetterGap)
			fg := ui.Mix(ui.Mix(ui.Blue, ui.Purple, float64(letter)/3), ui.FgWhite, float64(ri)*0.03)
			put(lx+i, ly+ri, r, ui.Style{FG: fg, BG: ui.BgTerm, Bold: true}.TCell())
		}
	}
}

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
		drawLogo(c, logoX, logoY, x, y, w, h)
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
