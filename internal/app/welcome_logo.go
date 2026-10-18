package app

import (
	"math"
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
	logoRamp      = "|&%$0=][{}<>*#+~^!?/\\;:"
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
func drawLogo(c ui.Canvas, lx, ly, x, y, w, h int, t float64) {
	put := func(cx, cy int, r rune, st tcell.Style) {
		if cx >= x && cx < x+w && cy >= y && cy < y+h {
			c.S.SetContent(cx, cy, r, nil, st)
		}
	}
	shadow := ui.Style{FG: ui.Mix(ui.BgTerm, ui.Blue, 0.12), BG: ui.BgTerm, Bold: true}.TCell()
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
			fg := ui.Mix(ui.Mix(ui.Blue, ui.Purple, float64(letter)/3), ui.FgWhite, float64(ri)*0.03)
			put(lx+i, ly+ri, logoGlyph(i, ri, t), ui.Style{FG: fg, BG: ui.BgTerm, Bold: true}.TCell())
		}
	}
}
