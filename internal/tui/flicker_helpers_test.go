package tui

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

// The flicker detector. Terminal flicker is a cell that shows one thing, then
// another, then the first again, or text that is repainted in a new color once
// it is already on screen. Both are visible in a sequence of frames, so the
// detector records the screen after every UI event and looks for them.

type frameCellState struct {
	r     rune
	comb  string
	style tcell.Style
}

type screenFrame struct {
	w, h  int
	cells []frameCellState
}

type frameSeq []screenFrame

func snapshotScreen(s tcell.SimulationScreen) screenFrame {
	w, h := s.Size()
	f := screenFrame{w: w, h: h, cells: make([]frameCellState, 0, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, comb, st, _ := s.GetContent(x, y)
			f.cells = append(f.cells, frameCellState{r, string(comb), st})
		}
	}
	return f
}

// cellPath is the distinct consecutive states of one cell over the frames.
func (fs frameSeq) cellPath(i int) []frameCellState {
	var path []frameCellState
	for _, f := range fs {
		if i >= len(f.cells) {
			continue
		}
		if len(path) == 0 || path[len(path)-1] != f.cells[i] {
			path = append(path, f.cells[i])
		}
	}
	return path
}

// reversals lists cells that went A, B, A. skip excludes regions that animate
// on purpose (clock text, the composer wave, the status bar).
func (fs frameSeq) reversals(skip func(x, y int) bool) []string {
	if len(fs) == 0 {
		return nil
	}
	w := fs[0].w
	var out []string
	for i := range fs[0].cells {
		x, y := i%w, i/w
		if skip != nil && skip(x, y) {
			continue
		}
		p := fs.cellPath(i)
		for k := 0; k+2 < len(p); k++ {
			if p[k+2] == p[k] {
				out = append(out, fmt.Sprintf("(%d,%d) %q back to %q", x, y, p[k+1].r, p[k].r))
				break
			}
		}
	}
	return out
}

// restyles counts, per cell, how often its glyph stayed and only its style
// changed: text that is already on screen repainted in other colors.
func (fs frameSeq) restyles(skip func(x, y int) bool) map[string]int {
	out := map[string]int{}
	if len(fs) == 0 {
		return out
	}
	w := fs[0].w
	for i := range fs[0].cells {
		x, y := i%w, i/w
		if skip != nil && skip(x, y) {
			continue
		}
		p := fs.cellPath(i)
		n := 0
		for k := 1; k < len(p); k++ {
			if p[k].r == p[k-1].r && p[k].comb == p[k-1].comb && p[k].style != p[k-1].style && p[k].r != ' ' {
				n++
			}
		}
		if n > 0 {
			out[fmt.Sprintf("(%d,%d)", x, y)] = n
		}
	}
	return out
}

func maxCount(m map[string]int) (string, int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	best, n := "", 0
	for _, k := range keys {
		if m[k] > n {
			best, n = k, m[k]
		}
	}
	return best, n
}

// pacedClient plays scripted turns with a pause between events, so the UI
// receives them one by one instead of as one burst.
type pacedClient struct {
	mu    sync.Mutex
	turns [][]llm.Event
	gap   time.Duration
}

func (c *pacedClient) Stream(ctx context.Context, _ llm.Request, emit func(llm.Event)) error {
	c.mu.Lock()
	var evs []llm.Event
	if len(c.turns) > 0 {
		evs, c.turns = c.turns[0], c.turns[1:]
	}
	c.mu.Unlock()
	for _, e := range evs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.gap):
		}
		emit(e)
	}
	return nil
}

// recordTurn types prompt, sends it and returns the screen after every UI event
// until the run ends. The timeline sits above the composer, which animates, so
// the returned skip function excludes the composer and the status bar.
func recordTurn(t *testing.T, a *App, scr tcell.SimulationScreen, prompt string) (frameSeq, func(x, y int) bool) {
	t.Helper()
	typeRunes(a, prompt)
	a.onKey(keyK(tcell.KeyEnter))
	var fs frameSeq
	deadline := time.After(10 * time.Second)
	for a.st.Running {
		select {
		case msg := <-a.events:
			a.handle(msg)
			a.draw()
			fs = append(fs, snapshotScreen(scr))
		case <-deadline:
			t.Fatal("the run did not end")
		}
	}
	a.draw()
	fs = append(fs, snapshotScreen(scr))
	top := a.st.ComposerY - 1
	return fs, func(x, y int) bool { return y >= top }
}
