package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

// composerMaxText caps how many text rows the composer may occupy.
const composerMaxText = 8

const (
	composerPadX = render.Gutter
	// composerPadY is the bottom padding; the top row is also padding.
	composerPadY = 1
)

// The three placeholders of an empty composer.
const (
	placeholderStart = "Type to start building anything"
	placeholderBusy  = "Tell the working agent something"
	placeholderNext  = "Type your next task here"
)

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
	return 1 + a.composerTextRows(w) + composerPadY
}

// placeholder is shown while the input is empty; it reflects what the agent is doing.
func (a *App) placeholder() string {
	if a.st.Ask != nil {
		return "answer the questions above..."
	}
	switch {
	case a.st.Running:
		return placeholderBusy
	case a.timelineEmpty():
		return placeholderStart
	}
	return placeholderNext
}

// cursorPos is the terminal cursor requested by the last draw.
type cursorPos struct {
	x, y int
	ok   bool
}

// applyCursor shows a real terminal cursor: a blinking bar in the composer,
// hidden while a selection is shown or a dialog is open.
func (a *App) applyCursor() {
	if !a.cur.ok || a.st.Over != OverNone || (a.askChoosing() && !a.st.Ask.Typing) || a.st.Selecting {
		a.scr.HideCursor()
		return
	}
	switch a.st.Mode {
	case render.ModeInput:
		a.scr.SetCursorStyle(tcell.CursorStyleBlinkingBar)
	default:
		a.scr.HideCursor()
		return
	}
	a.scr.ShowCursor(a.cur.x, a.cur.y)
}

// composerHeightFor is the composer band height for a screen width w.
func (a *App) composerHeightFor(w int) int {
	if a.panelOpen() && !a.typingAnswer() {
		_, h := a.scr.Size()
		if h < 10 {
			return 1
		}
		return 3
	}
	return 1 + min(composerMaxText, max(1, len(layoutInput(a.st.Input, a.composerInnerWidth(w))))) + composerPadY
}
