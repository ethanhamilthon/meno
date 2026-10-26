package tui

import (
	"github.com/gdamore/tcell/v2"
	"time"

	"meno/internal/render"
	"meno/internal/version"
)

const (
	welcomeBlockH = 19
	compactBlockH = 5
	tagline       = "Keep your agent simple."
	logoInterval  = time.Second / 6
)

var versionLabel = version.Label()

type animTick struct{}

type seg struct {
	text string
	fg   tcell.Color
	bold bool
}

func segsWidth(ss []seg) int {
	n := 0
	for _, s := range ss {
		n += render.TextWidth(s.text)
	}
	return n
}

// drawSegs draws the segments as one line centered in [x+Gutter, x+w-Gutter);
// a line that is too wide is cut at the right gutter.
func drawSegs(c render.Canvas, x, y, w int, ss []seg) {
	g := min(render.Gutter, max(0, (w-1)/2))
	lo, hi := x+g, x+w-g
	col := lo + max(0, (hi-lo-segsWidth(ss))/2)
	for _, s := range ss {
		st := render.Style{FG: s.fg, BG: render.BgTerm, Bold: s.bold}.TCell()
		col += c.Text(col, y, max(0, hi-col), s.text, st)
	}
}
