package tui

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
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
	logoRamp      = "|&%$0=][{}<>*#+~^!?/\\;:"
	tagline       = "thin and efficient AI coding harness"
)

// versionLabel is shown on the welcome screen and in the help title.
var versionLabel = version.Label()

// animTick wakes the render loop while the wordmark animates.
type animTick struct{}

// logoInterval is the frame time of the wordmark: its glyphs change six times a
// second (logoGlyph), so a faster tick would redraw identical frames.
const logoInterval = time.Second / 6

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

// logoGlyph is the symbol of one lit pixel at time t. The pixel grid stays put:
// only the glyphs change, six times a second.
func logoGlyph(col, row int, t float64) rune {
	step := uint32(int64(t * 6))
	h := uint32(col)*2654435761 ^ uint32(row)*2246822519 ^ step*3266489917
	h ^= h >> 15
	h *= 2246822519
	h ^= h >> 13
	ramp := []rune(logoRamp)
	return ramp[h%uint32(len(ramp))]
}

// drawLogo draws the wordmark at (lx, ly): a dim shadow one cell down and to
// the right, then the letters in a blue to purple gradient. The glyph of every
// lit pixel scrambles with t while the shape stays.
func drawLogo(c render.Canvas, lx, ly, x, y, w, h int, t float64) {
	put := func(cx, cy int, r rune, st tcell.Style) {
		if cx >= x && cx < x+w && cy >= y && cy < y+h {
			c.S.SetContent(cx, cy, r, nil, st)
		}
	}
	shadow := render.Style{FG: render.Mix(render.BgTerm, render.Blue, 0.12), BG: render.BgTerm, Bold: true}.TCell()
	for ri, row := range logoRows {
		for i, r := range []rune(row) {
			if r != ' ' {
				put(lx+i+1, ly+ri+1, logoGlyph(i, ri, t), shadow)
			}
		}
	}
	for ri, row := range logoRows {
		for i, r := range []rune(row) {
			if r == ' ' {
				continue
			}
			letter := i / (logoLetterW + logoLetterGap)
			fg := render.Mix(render.Mix(render.Blue, render.Purple, float64(letter)/3), render.FgWhite, float64(ri)*0.03)
			put(lx+i, ly+ri, logoGlyph(i, ri, t), render.Style{FG: fg, BG: render.BgTerm, Bold: true}.TCell())
		}
	}
}
