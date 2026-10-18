package app

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"

	"meno/internal/ui"
)

// composerMaxText caps how many text rows the composer may occupy.
const composerMaxText = 8

const placeholderIdle = "give a task... or use /skills"

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

func (a *App) composerInnerWidth(w int) int {
	pad := min(composerPadX, max(0, (w-1)/2))
	return max(1, w-2*pad)
}

// composerTextRows is the number of text rows currently shown in the composer.
func (a *App) composerTextRows(w int) int {
	if len(a.st.Input) == 0 {
		return 1
	}
	return min(composerMaxText, len(layoutInput(a.st.Input, a.composerInnerWidth(w))))
}

// composerHeight is the full composer band height including vertical padding.
func (a *App) composerHeight() int {
	w, _ := a.scr.Size()
	return a.composerTextRows(w) + composerPadY*2
}

// placeholder is shown while the input is empty; it reflects what the agent is doing.
func (a *App) placeholder() string {
	if a.st.Ask != nil {
		return "answer the questions above..."
	}
	if !a.st.Running {
		return placeholderIdle
	}
	text := a.st.Phase + "..."
	if a.st.Phase == "" {
		text = "working..."
	}
	if !a.st.RunStart.IsZero() {
		text += " " + clock(time.Since(a.st.RunStart))
	}
	return text
}

// cursorPos is the terminal cursor requested by the last draw.
type cursorPos struct {
	x, y int
	ok   bool
}

// applyCursor shows a real terminal cursor: blinking in insert mode, steady in
// normal and command mode, hidden in visual mode and while a dialog is open.
func (a *App) applyCursor() {
	if !a.cur.ok || a.st.Over != OverNone || a.askChoosing() {
		a.scr.HideCursor()
		return
	}
	switch a.st.Mode {
	case ui.ModeInsert:
		a.scr.SetCursorStyle(tcell.CursorStyleBlinkingBar)
	case ui.ModeNormal, ui.ModeVisual:
		a.scr.SetCursorStyle(tcell.CursorStyleSteadyBlock)
	case ui.ModeCommand:
		a.scr.SetCursorStyle(tcell.CursorStyleSteadyBar)
	default:
		a.scr.HideCursor()
		return
	}
	a.scr.ShowCursor(a.cur.x, a.cur.y)
}

// drawInputRow draws input[row.start:row.end] with prompt/skill highlighting.
func (a *App) drawInputRow(c ui.Canvas, x, y, maxW int, row vrow, toks []tokSpan, bg tcell.Color, mask []bool) {
	in := a.st.Input
	plain := ui.Style{FG: ui.FgWhite, BG: bg}.TCell()
	col, idx := 0, row.start
	g := uniseg.NewGraphemes(string(in[row.start:row.end]))
	for g.Next() && col < maxW {
		s := g.Str()
		st := plain
		for _, t := range toks {
			if idx >= t.start && idx < t.end {
				st = ui.Style{FG: tokenColor(t.kind), BG: bg}.TCell()
				break
			}
		}
		if idx < len(mask) && mask[idx] {
			st = st.Background(ui.BgSelect)
		}
		if s == "\t" {
			s = "    "
		}
		if r := []rune(s); len(r) == 1 {
			if o, ok := objectAt(r[0]); ok {
				s = o.Label()
				st = ui.Style{FG: ui.Blue, BG: bg}.TCell()
				if idx < len(mask) && mask[idx] {
					st = st.Background(ui.BgSelect)
				}
			}
		}
		col += c.Text(x+col, y, maxW-col, s, st)
		idx += len([]rune(g.Str()))
	}
}
