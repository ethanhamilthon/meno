package app

import (
	"context"

	"meno/internal/client"
	"meno/internal/proto"
	"meno/internal/ui"
)

// setAsk shows the form the daemon reports for a session, or removes it.
func (a *App) setAsk(rt *Runtime, f *proto.AskForm) {
	if f == nil {
		if rt.Ask != nil {
			a.closeAskFor(rt, rt.Ask)
		}
		return
	}
	if rt.Ask != nil && rt.Ask.ID == f.AskID {
		return
	}
	st := newAskState(*f)
	if len(st.F.Questions) == 0 {
		return
	}
	st.draft = draft{Input: rt.Input, Cursor: rt.Cursor, Mode: rt.Mode, Undo: rt.Undo, Redo: rt.Redo, InsBase: rt.InsBase}
	rt.Ask = st
	a.inRT(rt.ID(), func() {
		a.setInput(nil)
		a.st.Undo, a.st.Redo, a.st.InsBase = nil, nil, a.snapshot()
		a.st.Mode = ui.ModeInput
		a.st.Scroll = 0
	})
	a.signal("question")
}

// closeAsk removes the form and gives the composer its text back.
func (a *App) closeAsk(st *askState) { a.closeAskFor(a.st.Runtime, st) }

func (a *App) closeAskFor(rt *Runtime, st *askState) {
	if rt.Ask != st {
		return
	}
	rt.Ask = nil
	d := st.draft
	a.inRT(rt.ID(), func() {
		a.setInput(d.Input)
		a.st.Cursor = d.Cursor
		a.st.Undo, a.st.Redo, a.st.InsBase = d.Undo, d.Redo, d.InsBase
		a.st.Mode = d.Mode
		a.clampCursor()
	})
}

// answerAsk sends the answers (or a cancel) to the daemon and closes the form.
func (a *App) answerAsk(cancel bool) {
	st := a.st.Ask
	if st == nil {
		return
	}
	sid := a.st.Session.ID()
	req := proto.AskAnswerRequest{Cancel: cancel}
	if !cancel {
		req.Answers = st.build()
	}
	a.closeAsk(st)
	a.spawn(func() {
		if err := a.rpc(func(ctx context.Context, c *client.Client) error { return c.AnswerAsk(ctx, sid, st.ID, req) }); err != nil {
			a.postEvent(errNotice{text: "answer not delivered: " + err.Error()})
		}
	})
}
