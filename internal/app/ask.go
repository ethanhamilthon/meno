package app

import (
	"context"
	"strings"
	"time"

	"meno/internal/jsvm"
	"meno/internal/ui"
)

// Focus targets of the question form.
const (
	focusBody = iota
	focusPrev
	focusNext
	focusCancel
	focusCount
)

// askAnswer is the draft answer of one question, kept while the user moves
// around, resizes the terminal or comes back from another question.
type askAnswer struct {
	Picked []bool
	Other  []rune // custom text (the answer itself for a text question)
}

// askState is the form the agent waits on. It is shown as a dialog over the
// session; the composer's own text is put aside while it is open.
type askState struct {
	F      jsvm.Form
	Ans    []askAnswer
	Cur    int  // question on screen
	Focus  int  // focusBody...focusCancel
	Row    int  // cursor row in the body: options, then the custom-text row
	Typing bool // keys edit the custom text of the current question
	Notice string
	Until  time.Time
	reply  chan askResult
	draft  draft
	hits   []askHit // clickable areas of the last draw
}

type askResult struct {
	answers []jsvm.FormAnswer
	err     error
}

// draft is the composer as it was before the form.
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

func newAskState(f jsvm.Form) *askState {
	st := &askState{F: f, reply: make(chan askResult, 1)}
	for _, q := range f.Questions {
		st.Ans = append(st.Ans, askAnswer{Picked: make([]bool, len(q.Options))})
	}
	st.enter(0)
	if f.Timeout > 0 {
		st.Until = time.Now().Add(f.Timeout)
	}
	return st
}

// enter shows question i with the cursor on its selected row (or first row).
func (st *askState) enter(i int) {
	st.Cur = min(max(i, 0), len(st.F.Questions)-1)
	st.Row, st.Focus, st.Notice = 0, focusBody, ""
	q := st.F.Questions[st.Cur]
	st.Typing = q.Type == jsvm.TypeText
	if q.Type != jsvm.TypeText {
		for idx, picked := range st.Ans[st.Cur].Picked {
			if picked {
				st.Row = idx
				break
			}
		}
		if q.Type == jsvm.TypeSingle && strings.TrimSpace(string(st.Ans[st.Cur].Other)) != "" {
			st.Row = len(q.Options)
		}
	}
}

func (st *askState) q() jsvm.FormQuestion { return st.F.Questions[st.Cur] }

// rows is the number of body rows of the current question: the options and the
// custom-text row (a text question has only the text row).
func (st *askState) rows() int {
	if st.q().Type == jsvm.TypeText {
		return 1
	}
	return len(st.q().Options) + 1
}

// onOther reports whether the body cursor is on the custom-text row.
func (st *askState) onOther() bool {
	return st.q().Type == jsvm.TypeText || st.Row == len(st.q().Options)
}

// answered reports whether question i has a real answer. Empty custom text is
// not an answer.
func (st *askState) answered(i int) bool {
	a := st.Ans[i]
	if strings.TrimSpace(string(a.Other)) != "" {
		return true
	}
	if st.F.Questions[i].Type == jsvm.TypeText {
		return false
	}
	for _, p := range a.Picked {
		if p {
			return true
		}
	}
	return false
}

func (st *askState) answeredCount() int {
	n := 0
	for i := range st.Ans {
		if st.answered(i) {
			n++
		}
	}
	return n
}

// choose selects option row r of the current question. A single question holds
// one option or custom text, never both; a multi question holds both.
func (st *askState) choose(r int) {
	q := st.q()
	a := &st.Ans[st.Cur]
	if r < 0 || r >= len(q.Options) {
		return
	}
	switch q.Type {
	case jsvm.TypeSingle:
		for i := range a.Picked {
			a.Picked[i] = i == r
		}
		a.Other = nil
	case jsvm.TypeMulti:
		a.Picked[r] = !a.Picked[r]
	}
}

// build turns the drafts into answers: option IDs in input order, custom text
// separate, and text that belongs to an unselected alternative left out.
func (st *askState) build() []jsvm.FormAnswer {
	out := make([]jsvm.FormAnswer, len(st.F.Questions))
	for i, q := range st.F.Questions {
		a := st.Ans[i]
		ans := jsvm.FormAnswer{ID: q.ID, Selected: []string{}}
		for j, p := range a.Picked {
			if p {
				ans.Selected = append(ans.Selected, q.Options[j].ID)
			}
		}
		if text := strings.TrimSpace(string(a.Other)); text != "" && !(q.Type == jsvm.TypeSingle && len(ans.Selected) > 0) {
			ans.Text = text
		}
		out[i] = ans
	}
	return out
}

// appAsker implements jsvm.Asker: the VM runs on the agent's goroutine, the
// form lives on the UI goroutine, and the answers travel through a channel.
// Until the user submits, nothing is sent.
type appAsker struct {
	a   *App
	sid string // the session whose VM asks
}

func (k appAsker) Ask(ctx context.Context, f jsvm.Form) ([]jsvm.FormAnswer, error) {
	a := k.a
	st := newAskState(f)
	a.postEvent(askOpen{k.sid, st})
	select {
	case r := <-st.reply:
		return r.answers, r.err
	case <-ctx.Done():
		a.postEvent(askClose{k.sid, st})
		return nil, ctx.Err()
	case <-a.done:
		return nil, context.Canceled
	}
}

func (a *App) openAsk(st *askState) {
	if a.st.Ask != nil { // one form at a time; the VM is sequential
		st.reply <- askResult{err: jsvm.ErrCancelled}
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

// closeAsk removes the form and gives the composer its text back.
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

// askChoosing reports whether a form is open, so the composer stays out of it.
func (a *App) askChoosing() bool { return a.st.Ask != nil }

// askHeight: the form is a dialog, it takes no rows above the composer.
func (a *App) askHeight() int { return 0 }

// askStatus is the status-bar text while a form is open.
func (a *App) askStatus() string {
	if st := a.st.Ask; st != nil {
		return "waiting for your answers · " + itoa(st.answeredCount()) + "/" + itoa(len(st.F.Questions))
	}
	return ""
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
