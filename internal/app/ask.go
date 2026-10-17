package app

import (
	"context"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/jsvm"
	"meno/internal/ui"
)

// askState is the question the agent is waiting on, shown as a panel above the
// composer. The composer's own text is put aside while it is open.
type askState struct {
	Q      jsvm.Question
	Sel    int  // selected row of the choice list
	Typing bool // the answer is typed in the composer (free text or "your own answer")
	Until  time.Time
	reply  chan askResult
	draft  draft
}

type askResult struct {
	text string
	err  error
}

// draft is the composer as it was before the question.
type draft struct {
	Input      []rune
	Cursor     int
	Mode       ui.Mode
	Undo, Redo []snap
	InsBase    snap
}

type askOpen struct {
	sid string
	st  *askState
}
type askClose struct {
	sid string
	st  *askState
}

// appAsker implements jsvm.Asker: the VM runs on the agent's goroutine, the
// panel lives on the UI goroutine, and the answer travels through a channel.
type appAsker struct {
	a   *App
	sid string // the session whose VM asks
}

func (k appAsker) Ask(ctx context.Context, q jsvm.Question) (string, error) {
	a := k.a
	st := &askState{Q: q, reply: make(chan askResult, 1), Typing: len(q.Choices) == 0}
	if q.Timeout > 0 {
		st.Until = time.Now().Add(q.Timeout)
	}
	a.postEvent(askOpen{k.sid, st})
	select {
	case r := <-st.reply:
		return r.text, r.err
	case <-ctx.Done():
		a.postEvent(askClose{k.sid, st})
		return "", ctx.Err()
	case <-a.done:
		return "", context.Canceled
	}
}

func (a *App) openAsk(st *askState) {
	if a.st.Ask != nil { // one question at a time; the VM is sequential
		st.reply <- askResult{err: jsvm.ErrDeclined}
		return
	}
	st.draft = draft{Input: a.st.Input, Cursor: a.st.Cursor, Mode: a.st.Mode, Undo: a.st.Undo, Redo: a.st.Redo, InsBase: a.st.InsBase}
	a.st.Ask = st
	a.setInput(nil)
	a.st.Undo, a.st.Redo, a.st.InsBase = nil, nil, a.snapshot()
	if a.st.Mode != ui.ModeCommand {
		a.st.Mode = ui.ModeInsert
	}
	a.st.Scroll = 0
	a.signal("question")
}

// closeAsk removes the panel and gives the composer its text back.
func (a *App) closeAsk(st *askState) {
	if a.st.Ask != st {
		return
	}
	a.st.Ask = nil
	d := st.draft
	a.setInput(d.Input)
	a.st.Cursor = d.Cursor
	a.st.Undo, a.st.Redo, a.st.InsBase = d.Undo, d.Redo, d.InsBase
	if a.st.Mode != ui.ModeCommand {
		a.st.Mode = d.Mode
	}
	a.clampCursor()
}

func (a *App) finishAsk(res askResult) {
	st := a.st.Ask
	if st == nil {
		return
	}
	st.reply <- res
	a.closeAsk(st)
}

// askItems are the rows of the choice list.
func (st *askState) items() []string {
	out := append([]string(nil), st.Q.Choices...)
	if len(out) > 0 && st.Q.CustomAnswer {
		out = append(out, "Your own answer…")
	}
	return out
}

func (a *App) askChoosing() bool {
	return a.st.Ask != nil && !a.st.Ask.Typing
}

// askKey handles keys while a question is open; it reports whether it used the key.
func (a *App) askKey(k *tcell.EventKey) bool {
	st := a.st.Ask
	if k.Key() == tcell.KeyCtrlX {
		a.finishAsk(askResult{err: jsvm.ErrDeclined})
		return true
	}
	if st.Typing {
		return false // the composer takes it; submit() sends the answer
	}
	items := st.items()
	pick := func(i int) {
		if i < 0 || i >= len(items) {
			return
		}
		if i < len(st.Q.Choices) {
			a.finishAsk(askResult{text: st.Q.Choices[i]})
			return
		}
		st.Typing = true
		a.st.Mode = ui.ModeInsert
	}
	switch {
	case k.Key() == tcell.KeyEsc:
		a.finishAsk(askResult{err: jsvm.ErrDeclined})
	case k.Key() == tcell.KeyUp || k.Rune() == 'k':
		st.Sel = max(0, st.Sel-1)
	case k.Key() == tcell.KeyDown || k.Rune() == 'j':
		st.Sel = min(len(items)-1, st.Sel+1)
	case k.Key() == tcell.KeyEnter:
		pick(st.Sel)
	case k.Key() == tcell.KeyRune && k.Rune() >= '1' && k.Rune() <= '9':
		pick(int(k.Rune() - '1'))
	case k.Key() == tcell.KeyRune && k.Rune() == ':':
		a.st.Mode = ui.ModeNormal
		return false
	}
	return true
}

// answerTyped sends the composer text as the answer.
func (a *App) answerTyped() {
	text := strings.TrimSpace(string(a.st.Input))
	if text == "" {
		return
	}
	a.finishAsk(askResult{text: text})
}

const askMaxShare = 40 // percent of the screen height

// askLayout splits the panel into question rows and choice rows for a panel of
// at most limit rows (header and hint included).
func (a *App) askLayout(w, limit int) (q []string, choices, total int) {
	st := a.st.Ask
	if st == nil || limit < 3 || w < 8 {
		return nil, 0, 0
	}
	q = wordWrap(st.Q.Text, max(1, w-4))
	avail := limit - 2 // header and hint
	n := 0
	if !st.Typing {
		n = len(st.items())
	}
	// The question comes first, but at least two choices stay visible.
	qRows := max(1, min(len(q), avail-min(n, 2)))
	choices = max(0, min(n, avail-qRows))
	if qRows < len(q) {
		q = append(q[:qRows-1:qRows-1], ui.Truncate(q[qRows-1]+" …", max(1, w-4)))
	}
	return q, choices, 2 + qRows + choices
}

// askHeight is the rows the panel takes above the composer.
func (a *App) askHeight() int {
	if a.st.Ask == nil || a.st.Over != OverNone || a.st.Mode == ui.ModeCommand {
		return 0
	}
	w, h := a.scr.Size()
	_, _, total := a.askLayout(w, max(5, h*askMaxShare/100))
	return total
}

func (a *App) drawAsk(c ui.Canvas, x, y, w, limit int) {
	st := a.st.Ask
	q, nChoices, total := a.askLayout(w, limit)
	if total == 0 || st == nil {
		return
	}
	bg := ui.BgPopup
	c.Fill(x, y, w, total, bg)
	style := func(fg tcell.Color, bg tcell.Color) tcell.Style {
		return tcell.StyleDefault.Foreground(fg).Background(bg)
	}
	c.Text(x+2, y, max(0, w-4), "? agent asks", style(ui.Orange, bg))
	if !st.Until.IsZero() {
		left := time.Until(st.Until)
		right := "answer within " + fmtWait(max(left, 0))
		c.Text(x+max(2, w-2-ui.TextWidth(right)), y, ui.TextWidth(right), right, style(ui.FgDim, bg))
	}
	row := y + 1
	for _, l := range q {
		c.Text(x+2, row, max(0, w-4), l, style(ui.FgWhite, bg))
		row++
	}
	if nChoices > 0 {
		items := st.items()
		start := 0
		if st.Sel >= nChoices {
			start = st.Sel - nChoices + 1
		}
		for i := 0; i < nChoices; i++ {
			idx := start + i
			rbg, fg, mark := bg, ui.FgSoft, " "
			if idx == st.Sel {
				rbg, fg, mark = ui.BgPopupSel, ui.FgWhite, "▸"
			}
			c.Fill(x, row, w, 1, rbg)
			c.Text(x+2, row, 2, mark, style(ui.Blue, rbg))
			num := " "
			if idx < 9 {
				num = string(rune('1' + idx))
			}
			c.Text(x+4, row, 2, num, style(ui.FgDim, rbg))
			c.Text(x+6, row, max(0, w-8), items[idx], style(fg, rbg))
			row++
		}
	}
	hint := a.askHint(w)
	c.Fill(x, y+total-1, w, 1, bg)
	if more := len(st.items()) - nChoices; nChoices > 0 && more > 0 {
		label := "↕ " + itoa(more) + " more"
		if ui.TextWidth(hint)+ui.TextWidth(label)+6 <= w {
			c.Text(x+w-2-ui.TextWidth(label), y+total-1, ui.TextWidth(label), label, style(ui.FgDim, bg))
		}
	}
	c.Text(x+2, y+total-1, max(0, w-4), hint, style(ui.FgDim, bg))
}

func (a *App) askHint(w int) string {
	st := a.st.Ask
	var full, short string
	switch {
	case st.Typing && len(st.Q.Choices) > 0:
		full, short = "type your answer · enter send · ctrl+x decline", "enter send · ^x decline"
	case st.Typing:
		full, short = "type your answer below · enter send · ctrl+x decline", "enter send · ^x decline"
	default:
		full, short = "↑/↓ or 1-9 choose · enter confirm · esc decline", "↑/↓ enter esc"
	}
	if ui.TextWidth(full) > w-4 {
		return short
	}
	return full
}

// askStatus is the status-bar text while a question is open.
func (a *App) askStatus() string {
	if a.st.Ask == nil {
		return ""
	}
	return "waiting for your answer"
}

// wordWrap wraps text to width columns at spaces, keeping line breaks; words
// longer than a row are cut.
func wordWrap(text string, width int) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			for ui.TextWidth(word) > width {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				parts := ui.Wrap(word, width)
				out = append(out, parts[:len(parts)-1]...)
				word = parts[len(parts)-1]
			}
			switch {
			case line == "":
				line = word
			case ui.TextWidth(line)+1+ui.TextWidth(word) <= width:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
		}
		out = append(out, line)
	}
	return out
}
