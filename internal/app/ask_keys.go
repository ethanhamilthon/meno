package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/jsvm"
)

// askKey handles every key while a form is open: nothing reaches the composer.
// Esc never closes the form; only the Cancel action does.
func (a *App) askKey(k *tcell.EventKey) {
	st := a.st.Ask
	if st.Typing {
		a.askTypingKey(st, k)
		return
	}
	switch k.Key() {
	case tcell.KeyTab:
		st.Focus = (st.Focus + 1) % focusCount
		return
	case tcell.KeyBacktab:
		st.Focus = (st.Focus + focusCount - 1) % focusCount
		return
	case tcell.KeyEsc:
		st.Focus = focusBody // leave a nested focus; the form stays open
		return
	case tcell.KeyPgUp:
		a.askMove(-1)
		return
	case tcell.KeyPgDn:
		a.askMove(1)
		return
	}
	if st.Focus != focusBody {
		if k.Key() == tcell.KeyEnter || k.Rune() == ' ' {
			a.askActivate(st.Focus)
		} else if k.Key() == tcell.KeyLeft || isKey(k, 'h') {
			st.Focus = max(focusBody, st.Focus-1)
		} else if k.Key() == tcell.KeyRight || isKey(k, 'l') {
			st.Focus = min(focusCancel, st.Focus+1)
		} else if k.Key() == tcell.KeyUp || isKey(k, 'k') {
			st.Focus = focusBody
		}
		return
	}
	switch {
	case k.Key() == tcell.KeyUp || isKey(k, 'k'):
		st.Row = max(0, st.Row-1)
	case k.Key() == tcell.KeyDown || isKey(k, 'j'):
		st.Row = min(st.rows()-1, st.Row+1)
	case k.Key() == tcell.KeyLeft || isKey(k, 'h') || isKey(k, '['):
		a.askMove(-1)
	case k.Key() == tcell.KeyRight || isKey(k, 'l') || isKey(k, ']'):
		a.askMove(1)
	case k.Key() == tcell.KeyEnter || k.Rune() == ' ':
		a.askPressRow()
	case k.Key() == tcell.KeyRune && k.Rune() >= '1' && k.Rune() <= '9':
		if r := int(k.Rune() - '1'); r < len(st.q().Options) {
			st.Row = r
			a.askPressRow()
		}
	}
}

// askPressRow acts on the body row under the cursor.
func (a *App) askPressRow() {
	st := a.st.Ask
	if st.onOther() {
		st.Typing = true
		return
	}
	st.choose(st.Row)
	st.Notice = ""
}

func (a *App) askMove(d int) {
	st := a.st.Ask
	if n := st.Cur + d; n >= 0 && n < len(st.F.Questions) {
		st.enter(n)
	}
}

// askActivate runs a button.
func (a *App) askActivate(focus int) {
	st := a.st.Ask
	switch focus {
	case focusPrev:
		a.askMove(-1)
	case focusNext:
		a.askMove(1)
	case focusSubmit:
		a.askSubmit()
	case focusCancel:
		a.finishAsk(askResult{err: jsvm.ErrCancelled})
	}
	_ = st
}

// askSubmit validates every question; the first unanswered one gets the focus
// and nothing is sent.
func (a *App) askSubmit() {
	st := a.st.Ask
	for i := range st.F.Questions {
		if !st.answered(i) {
			st.enter(i)
			st.Notice = "answer this question first (" + itoa(i+1) + " of " + itoa(len(st.F.Questions)) + ")"
			return
		}
	}
	a.finishAsk(askResult{answers: st.build()})
}

// askTypingKey edits the custom text of the current question.
func (a *App) askTypingKey(st *askState, k *tcell.EventKey) {
	ans := &st.Ans[st.Cur]
	switch k.Key() {
	case tcell.KeyEsc, tcell.KeyEnter:
		if k.Key() == tcell.KeyEnter && k.Modifiers()&(tcell.ModShift|tcell.ModAlt) != 0 {
			a.askInsert("\n")
			return
		}
		if st.q().Type != jsvm.TypeText {
			st.Typing = false
		} else if k.Key() == tcell.KeyEsc {
			st.Typing, st.Focus = false, focusSubmit
		} else {
			st.Typing, st.Focus = false, focusNext
		}
	case tcell.KeyTab:
		st.Typing = false
		st.Focus = (st.Focus + 1) % focusCount
	case tcell.KeyBacktab:
		st.Typing = false
		st.Focus = (st.Focus + focusCount - 1) % focusCount
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n := len(ans.Other); n > 0 {
			ans.Other = ans.Other[:n-1]
		}
	case tcell.KeyCtrlU:
		ans.Other = nil
	case tcell.KeyCtrlJ:
		a.askInsert("\n")
	case tcell.KeyRune:
		a.askInsert(string(k.Rune()))
	}
}

// askInsert appends text to the custom text of the current question (typing or
// pasting). In a single question, custom text replaces a chosen option.
func (a *App) askInsert(text string) {
	st := a.st.Ask
	if st == nil || strings.TrimSpace(text) == "" && len(st.Ans[st.Cur].Other) == 0 {
		return
	}
	ans := &st.Ans[st.Cur]
	ans.Other = append(ans.Other, []rune(text)...)
	if st.q().Type == jsvm.TypeSingle && strings.TrimSpace(string(ans.Other)) != "" {
		for i := range ans.Picked {
			ans.Picked[i] = false
		}
	}
	st.Notice = ""
}
