package agent

import (
	"context"
	"errors"
	"fmt"

	"meno/internal/jsvm"
	"meno/internal/llm"
)

const MaxSteps = 25

type Ev struct {
	Kind     string
	Text     string
	Thinking string
	Code     string
	Output   string
	Err      string
	CallID   string
	Usage    llm.Usage
}

const (
	EvText    = "text"
	EvThink   = "think"
	EvJSStart = "js_start"
	EvJSEnd   = "js_end"
	EvDone    = "done"
	EvError   = "error"
	EvAborted = "aborted"
)

type Deps struct {
	Client   llm.Client
	Provider llm.Provider
	Model    string
	Effort   string
	System   string
	VM       *jsvm.VM
	Thinking bool
	Emit     func(Ev)
}

type Runner struct {
	D Deps
}

func (r *Runner) Turn(ctx context.Context, s *Session, userText string) error {
	s.Append(Turn{Role: llm.RoleUser, Text: userText})
	for step := 0; step < MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return r.abort(s)
		}
		calls, text, err := r.streamOnce(ctx, s)
		if errors.Is(err, context.Canceled) {
			return r.abort(s)
		}
		if err != nil {
			r.D.Emit(Ev{Kind: EvError, Err: err.Error()})
			return err
		}
		if len(calls) == 0 {
			s.Append(Turn{Role: llm.RoleAssistant, Text: text})
			r.D.Emit(Ev{Kind: EvDone})
			return nil
		}
		for _, c := range calls {
			if err := r.runJS(ctx, s, text, c); err != nil {
				return r.abort(s)
			}
			text = ""
		}
	}
	err := fmt.Errorf("step limit %d reached", MaxSteps)
	r.D.Emit(Ev{Kind: EvError, Err: err.Error()})
	return err
}

func (r *Runner) abort(s *Session) error {
	s.Append(Turn{Role: llm.RoleAssistant, Text: "[aborted]"})
	r.D.Emit(Ev{Kind: EvAborted})
	return context.Canceled
}

type call struct {
	ID   string
	Code string
}

func (r *Runner) streamOnce(ctx context.Context, s *Session) ([]call, string, error) {
	var calls []call
	var text string
	var streamErr error
	err := r.D.Client.Stream(ctx, llm.Request{
		Provider: r.D.Provider, Model: r.D.Model, Effort: r.D.Effort,
		System: r.D.System, Msgs: s.ToMsgs(), Tools: true,
	}, func(e llm.Event) {
		switch e.Kind {
		case llm.EvText:
			text += e.Text
			r.D.Emit(Ev{Kind: EvText, Text: e.Text})
		case llm.EvThinking:
			if r.D.Thinking {
				r.D.Emit(Ev{Kind: EvThink, Thinking: e.Text})
			}
		case llm.EvTool:
			calls = append(calls, call{ID: e.CallID, Code: e.Code})
		case llm.EvError:
			streamErr = errors.New(e.Text)
		case llm.EvDone:
			r.D.Emit(Ev{Kind: "usage", Usage: e.Usage})
		}
	})
	if err == nil {
		err = streamErr
	}
	return calls, text, err
}

func (r *Runner) runJS(ctx context.Context, s *Session, text string, c call) error {
	if c.ID == "" {
		c.ID = NewID()
	}
	r.D.Emit(Ev{Kind: EvJSStart, Code: c.Code, CallID: c.ID})
	res := r.D.VM.Exec(ctx, c.Code)
	turn := Turn{Role: llm.RoleAssistant, Text: text, CallID: c.ID, Code: c.Code, Output: res.Output, Err: res.Err, Time: nowUTC()}
	if res.Err == "" && res.Value != "" {
		turn.Output += res.Value
	}
	s.Append(turn)
	r.D.Emit(Ev{Kind: EvJSEnd, CallID: c.ID, Output: turn.Output, Err: res.Err})
	return ctx.Err()
}
