package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"meno/internal/api"
	"meno/internal/form"
	"time"
)

type asker struct {
	e *Engine
	s *runState
}

func (h asker) Ask(ctx context.Context, f form.Form) ([]form.Answer, error) {
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	st := h.s
	st.mu.Lock()
	if st.ask != nil {
		st.mu.Unlock()
		return nil, dapi(api.CodeConflict, "another question is already open")
	}
	a := &askState{id: id, form: f, answer: make(chan askReply, 1), by: st.session.ID, deadline: time.Time{}}
	if f.Timeout > 0 {
		a.deadline = time.Now().Add(f.Timeout)
	}
	st.ask = a
	st.phase = "asking"
	st.mu.Unlock()
	h.e.publish(st.session.ID, api.EventAskOpen, askWire(a))
	defer func() {
		st.mu.Lock()
		if st.ask == a {
			st.ask = nil
			st.phase = "thinking"
		}
		st.mu.Unlock()
		h.e.publish(st.session.ID, api.EventAskClosed, protoAskClosed(id, ""))
	}()
	if f.Timeout > 0 {
		timer := time.NewTimer(f.Timeout)
		defer timer.Stop()
		select {
		case r := <-a.answer:
			return r.answers, r.err
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, form.ErrTimeout
		}
	}
	select {
	case r := <-a.answer:
		return r.answers, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func protoAskClosed(id, by string) any { return api.AskClosedData{AskID: id, By: by} }
