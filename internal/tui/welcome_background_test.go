package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

// welcomeLayout returns where the welcome content sits for a w x h screen: the
// first row of the block, the height of the log area and the wordmark box with
// its one-cell shadow.
func welcomeLayout(a *App, w, h int) (top, logH, boxX, boxY int) {
	logH = h - a.bottomHeight()
	top = (logH - welcomeBlockH) / 2
	return top, logH, 0, top + 6
}

// welcomeStrays lists the cells of the log area that hold anything but a blank
// terminal cell, outside the wordmark box and the four text rows.
func welcomeStrays(a *App, s tcell.SimulationScreen, w, h int) []string {
	top, logH, _, _ := welcomeLayout(a, w, h)
	textRows := map[int]bool{top + 6: true, top + 8: true, top + 15: true, top + 18: true}
	var out []string
	for y := 0; y < logH; y++ {
		if textRows[y] {
			continue
		}
		for x := 0; x < w; x++ {
			r, _, st, _ := s.GetContent(x, y)
			_, bg, _ := st.Decompose()
			if r != ' ' || bg != render.BgTerm {
				out = append(out, fmt.Sprintf("(%d,%d)=%q", x, y, r))
			}
		}
	}
	return out
}

func TestWelcomeBackgroundIsBlank(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	base := time.Now()
	for _, dt := range []time.Duration{0, 400 * time.Millisecond, 1300 * time.Millisecond} {
		a.now = func() time.Time { return base.Add(dt) }
		a.draw()
		if strays := welcomeStrays(a, s, 112, 36); len(strays) != 0 {
			t.Fatalf("at +%v: %d background cells, first %v", dt, len(strays), strays[:min(5, len(strays))])
		}
	}
}

func TestWelcomeBackgroundIsBlankWhenCompact(t *testing.T) {
	a, s := visualFixture(t, 60, 14)
	a.draw()
	_, logH, _, _ := welcomeLayout(a, 60, 14)
	top := (logH - compactBlockH) / 2
	for y := 0; y < logH; y++ {
		if y >= top && y < top+compactBlockH {
			continue
		}
		for x := 0; x < 60; x++ {
			if r, _, _, _ := s.GetContent(x, y); r != ' ' {
				t.Fatalf("compact welcome has %q at (%d,%d)", r, x, y)
			}
		}
	}
}

// runesOf copies the glyph of every cell; GetContents hands out the live buffer.
func runesOf(s tcell.SimulationScreen) []rune {
	w, h := s.Size()
	out := make([]rune, 0, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, _, _, _ := s.GetContent(x, y)
			out = append(out, r)
		}
	}
	return out
}

func TestWelcomeHeaderDoesNotAnimate(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	base := time.Now()
	a.now = func() time.Time { return base }
	a.draw()
	if a.animOn.Load() {
		t.Fatal("static welcome requested animation")
	}
	before := runesOf(s)
	a.now = func() time.Time { return base.Add(time.Second) }
	a.draw()
	after := runesOf(s)
	for i := range before {
		if before[i] != after[i] {
			t.Fatal("static welcome changed")
		}
	}
}
