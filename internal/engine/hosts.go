package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"meno/internal/jsvm"
	"meno/internal/proto"
	"time"
)

type goalHost struct{ s *runState }

func (h goalHost) Done(x string) error {
	if h.s.goal == nil {
		return errors.New("goal is unavailable")
	}
	return h.s.goal.Done(x)
}
func (h goalHost) Blocked(x string) error {
	if h.s.goal == nil {
		return errors.New("goal is unavailable")
	}
	return h.s.goal.Blocked(x)
}

type asker struct {
	e *Engine
	s *runState
}

func (h asker) Ask(ctx context.Context, f jsvm.Form) ([]jsvm.FormAnswer, error) {
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	st := h.s
	st.mu.Lock()
	a := &askState{id: id, form: f, answer: make(chan askReply, 1), by: st.session.ID, deadline: time.Time{}}
	if f.Timeout > 0 {
		a.deadline = time.Now().Add(f.Timeout)
	}
	st.ask = a
	st.phase = "asking"
	st.mu.Unlock()
	h.e.publish(st.session.ID, proto.EventAskOpen, askWire(a))
	defer func() {
		st.mu.Lock()
		if st.ask == a {
			st.ask = nil
		}
		st.mu.Unlock()
		h.e.publish(st.session.ID, proto.EventAskClosed, protoAskClosed(id, ""))
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
			return nil, jsvm.ErrTimeout
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
func protoAskClosed(id, by string) any { return proto.AskClosedData{AskID: id, By: by} }
