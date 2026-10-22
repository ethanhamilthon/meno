package tui

import (
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

// mouseState tracks a press-drag-release on the timeline. A drag selects text
// like vim's visual mode (the selection is the visual selection), a double
// click selects a word, a triple click a line. The selection is copied to the
// clipboard when the button is released and stays highlighted until the next
// key or click.
type mouseState struct {
	down, dragging bool
	input          bool // the drag began in the composer
	clicks         int
	at             time.Time
	x, y           int
}

const doubleClick = 400 * time.Millisecond

func (a *App) onMouse(m *tcell.EventMouse) {
	if a.st.Over != OverNone {
		return
	}
	if a.st.Ask != nil {
		a.askMouse(m)
		return
	}
	b := m.Buttons()
	switch {
	case b&tcell.WheelUp != 0:
		a.scrollBy(3)
		return
	case b&tcell.WheelDown != 0:
		a.scrollBy(-3)
		return
	}
	x, y := m.Position()
	if b&tcell.Button1 != 0 && !a.mouse.down {
		if _, h := a.scr.Size(); y == h-1 {
			if f := a.counterAt(x); f > filterAll {
				a.openSessionsFiltered(f)
				return
			}
		}
	}
	switch {
	case b&tcell.Button1 != 0 && !a.mouse.down:
		a.mousePress(x, y)
	case b&tcell.Button1 != 0:
		a.mouseDrag(x, y)
	case a.mouse.down:
		a.mouseRelease()
	}
}

// logHeight is the number of screen rows of the timeline.
func (a *App) logHeight() int {
	_, h := a.scr.Size()
	return h - min(h, a.bottomHeight())
}

// logPos maps a screen cell to a timeline position. ok is false outside the
// timeline text; with clamp the row is pulled into the timeline instead (for
// drags that leave it).
func (a *App) logPos(x, y int, clamp bool) (p vpos, ok bool) {
	logH := a.logHeight()
	if a.timelineEmpty() || logH <= 0 {
		return vpos{}, false
	}
	if clamp {
		y = min(max(y, 0), logH-1)
	} else if y < 0 || y >= logH {
		return vpos{}, false
	}
	w, _ := a.scr.Size()
	lines := a.transcriptWithSelection(w)
	i := max(0, len(lines)-logH-a.st.Scroll) + y
	if i >= len(lines) {
		if !clamp || len(lines) == 0 {
			return vpos{}, false
		}
		i = len(lines) - 1
	}
	return vpos{i, textCol([]rune(lines[i].Text()), x-render.PadX)}, true
}

// mousePos maps a screen cell to a document position, in the timeline or in
// the composer.
func (a *App) mousePos(x, y int, clamp bool) (vpos, bool) {
	if y >= a.logHeight() {
		return a.inputPosAt(x, y, clamp)
	}
	return a.logPos(x, y, clamp)
}

// inputPosAt maps a screen cell to a composer line and rune column. The
// geometry comes from the last drawComposer call.
func (a *App) inputPosAt(x, y int, clamp bool) (vpos, bool) {
	rows := a.st.ComposerRows
	if len(rows) == 0 {
		return vpos{}, false
	}
	idx := y - a.st.ComposerY + a.st.ComposerTop
	if idx < 0 || idx >= len(rows) {
		if !clamp {
			return vpos{}, false
		}
		idx = min(max(idx, 0), len(rows)-1)
	}
	row := rows[idx]
	lo, hi := min(row.start, len(a.st.Input)), min(row.end, len(a.st.Input))
	col := inputColAt(a.st.Input[lo:hi], x-a.st.ComposerX)
	// Composer rows wrap; document lines do not. The start of a wrapped row
	// gives its logical line and column.
	in := a.inputText()
	p := in.pos(lo)
	w, _ := a.scr.Size()
	return vpos{len(a.transcriptWithSelection(w)) + p.line, p.col + col}, true
}

// inputColAt is the rune index covered by display column col.
func inputColAt(rs []rune, col int) int {
	if col <= 0 {
		return 0
	}
	acc := 0
	for i, r := range rs {
		acc += cellWidth(string(r))
		if col < acc {
			return i
		}
	}
	return len(rs)
}

// textCol is the index of the rune that covers display column col.
func textCol(rs []rune, col int) int {
	if col <= 0 {
		return 0
	}
	acc := 0
	for i, r := range rs {
		acc += render.TextWidth(string(r))
		if col < acc {
			return i
		}
	}
	return len(rs)
}

func (a *App) mousePress(x, y int) {
	p, ok := a.mousePos(x, y, false)
	if !ok {
		return
	}
	a.mouse.input = y >= a.logHeight()
	if a.st.Mode == render.ModeInput {
		a.endInsert()
	}
	if a.st.Mode == render.ModeVisual {
		a.exitVisual()
	}
	now := time.Now()
	if a.mouse.clicks > 0 && now.Sub(a.mouse.at) < doubleClick && a.mouse.x == x && a.mouse.y == y {
		a.mouse.clicks = a.mouse.clicks%3 + 1
	} else {
		a.mouse.clicks = 1
	}
	a.mouse.down, a.mouse.dragging, a.mouse.at, a.mouse.x, a.mouse.y = true, false, now, x, y
	t, top := a.doc()
	g := t.clamp(p, false)
	a.st.VimBuf = nil
	a.setCurG(g, top)
	switch a.mouse.clicks {
	case 2:
		a.selectWordAt(t, top, g)
	case 3:
		a.startVisual('V')
	}
}

func (a *App) mouseDrag(x, y int) {
	if !a.mouse.input {
		if logH := a.logHeight(); y <= 0 {
			a.scrollBy(1)
		} else if y >= logH-1 {
			a.scrollBy(-1)
		}
	}
	p, ok := a.mousePos(x, y, true)
	if !ok {
		return
	}
	t, top := a.doc()
	g := t.clamp(p, false)
	if !a.mouse.dragging {
		if g == a.curG(top) {
			return
		}
		a.mouse.dragging = true
	}
	if a.st.Mode != render.ModeVisual {
		a.startVisual('v') // the anchor is the cell that was pressed
	}
	a.setCurG(g, top)
}

func (a *App) mouseRelease() {
	a.mouse.down = false
	if a.st.Mode != render.ModeVisual {
		return
	}
	t, top := a.doc()
	text := joinRngs(t, a.selRngs(t, top))
	a.st.SelText = text
	a.setReg(text, a.st.VKind == 'V')
	a.notify(fmt.Sprintf("copied %d characters", utf8.RuneCountInString(text)))
}

// selectWordAt selects the run of the same kind of characters (word, space,
// punctuation) around g.
func (a *App) selectWordAt(t vtext, top int, g vpos) {
	rs := t[g.line]
	if g.col >= len(rs) {
		return
	}
	kind := runeKind(rs[g.col])
	lo, hi := g.col, g.col
	for lo > 0 && runeKind(rs[lo-1]) == kind {
		lo--
	}
	for hi+1 < len(rs) && runeKind(rs[hi+1]) == kind {
		hi++
	}
	a.startVisual('v')
	a.setAnchorG(vpos{g.line, lo}, top)
	a.setCurG(vpos{g.line, hi}, top)
}

func runeKind(r rune) int {
	switch {
	case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
		return 0
	case unicode.IsSpace(r):
		return 1
	}
	return 2
}
