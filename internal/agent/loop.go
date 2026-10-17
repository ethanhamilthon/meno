package agent

import (
	"context"
	"errors"
	"time"

	"meno/internal/jsvm"
	"meno/internal/llm"
)

type Ev struct {
	Kind     string
	Text     string
	Thinking string
	Code     string
	Output   string
	Err      string
	CallID   string
	Usage    llm.Usage
	Provider llm.Provider
	Model    string
	// Turns is the session length when the event was emitted, so the UI can tell
	// live (streaming) content from content that is already persisted.
	Turns int
}

const (
	EvText    = "text"
	EvThink   = "think"
	EvCode    = "code" // tool call code streaming in (Code is the full text so far)
	EvJSStart = "js_start"
	EvJSEnd   = "js_end"
	EvDone    = "done"
	EvUsage   = "usage"
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
	// ModelTimeout lets the model pass a timeout with every js call.
	ModelTimeout bool
	Emit         func(Ev)
}

type Runner struct {
	D Deps
}

func (r *Runner) Turn(ctx context.Context, s *Session, userText string) error {
	s.Append(Turn{Role: llm.RoleUser, Text: userText})
	return r.Continue(ctx, s)
}

func (r *Runner) Continue(ctx context.Context, s *Session) error {
	// No step limit: the loop ends when the model stops calling tools, on error,
	// or when ctx is canceled (ctrl+c).
	for {
		if err := ctx.Err(); err != nil {
			return r.abort(s)
		}
		calls, replay, err := r.streamOnce(ctx, s)
		text := replay.Text
		if err != nil {
			if len(calls) != 0 {
				group := NewID()
				for i, c := range calls {
					var history *llm.Msg
					if i == 0 {
						history = &replay
					}
					s.Append(Turn{Role: llm.RoleAssistant, Text: text, CallID: c.ID, Code: c.Code, Err: "not executed: " + err.Error(), Replay: history, ReplayGroup: group})
					text = ""
				}
			} else if text != "" || len(replay.Responses) != 0 || replay.Thought != "" {
				s.Append(Turn{Role: llm.RoleAssistant, Text: text, Replay: &replay})
			}
		}
		if errors.Is(err, context.Canceled) {
			return r.abort(s)
		}
		if err != nil {
			r.emit(s, Ev{Kind: EvError, Err: err.Error()})
			return err
		}
		if len(calls) == 0 {
			s.Append(Turn{Role: llm.RoleAssistant, Text: text, Replay: &replay})
			r.emit(s, Ev{Kind: EvDone})
			return nil
		}
		group := NewID()
		for i, c := range calls {
			var history *llm.Msg
			if i == 0 {
				history = &replay
			}
			if err := r.runJS(ctx, s, text, c, history, group); err != nil {
				// Every announced call needs an output, including unexecuted calls.
				for _, pending := range calls[i+1:] {
					s.Append(Turn{Role: llm.RoleAssistant, CallID: pending.ID, Code: pending.Code, Err: "execution canceled", ReplayGroup: group})
				}
				return r.abort(s)
			}
			text = ""
		}
	}
}

func (r *Runner) emit(s *Session, e Ev) {
	e.Turns = s.Len()
	r.D.Emit(e)
}

func (r *Runner) abort(s *Session) error {
	s.Append(Turn{Role: llm.RoleAssistant, Text: "[aborted]"})
	r.emit(s, Ev{Kind: EvAborted})
	return context.Canceled
}

type call struct {
	ID      string
	Code    string
	Timeout *float64 // seconds, when the model asked for one
}

func (r *Runner) streamOnce(ctx context.Context, s *Session) ([]call, llm.Msg, error) {
	var calls []call
	replay := llm.Msg{Role: llm.RoleAssistant}
	var streamErr error
	err := r.D.Client.Stream(ctx, llm.Request{
		Provider: r.D.Provider, Model: r.D.Model, Effort: r.D.Effort,
		System: r.D.System, Msgs: s.ToMsgs(), Tools: true, ModelTimeout: r.D.ModelTimeout,
	}, func(e llm.Event) {
		switch e.Kind {
		case llm.EvText:
			replay.Text += e.Text
			r.emit(s, Ev{Kind: EvText, Text: e.Text})
		case llm.EvThinking:
			replay.Thought += e.Text
			if r.D.Thinking {
				r.emit(s, Ev{Kind: EvThink, Thinking: e.Text})
			}
		case llm.EvToolDelta:
			r.emit(s, Ev{Kind: EvCode, CallID: e.CallID, Code: e.Code})
		case llm.EvTool:
			id := e.CallID
			if id == "" {
				id = NewID()
			}
			name := e.Name
			if name == "" {
				name = llm.ToolName
			}
			args := e.Arguments
			if args == "" {
				legacy := llm.Msg{Role: llm.RoleAssistant, CallID: id, Code: e.Code}
				args = legacy.Calls()[0].Arguments
			}
			c := call{ID: id, Code: e.Code}
			if r.D.ModelTimeout && e.Arguments != "" {
				_, c.Timeout = llm.ToolArgs(e.Arguments)
			}
			calls = append(calls, c)
			replay.ToolCalls = append(replay.ToolCalls, llm.ToolCall{ID: id, Name: name, Arguments: args})
			if len(e.Item) != 0 {
				replay.Responses = append(replay.Responses, e.Item)
			}
		case llm.EvHistory:
			if len(e.Item) != 0 {
				replay.Responses = append(replay.Responses, e.Item)
			}
		case llm.EvError:
			streamErr = errors.New(e.Text)
		case llm.EvDone:
			r.emit(s, Ev{Kind: EvUsage, Usage: e.Usage, Provider: r.D.Provider, Model: r.D.Model})
		}
	})
	if err == nil {
		err = streamErr
	}
	return calls, replay, err
}

func (r *Runner) runJS(ctx context.Context, s *Session, text string, c call, replay *llm.Msg, group string) error {
	if c.ID == "" {
		c.ID = NewID()
	}
	r.emit(s, Ev{Kind: EvJSStart, Code: c.Code, CallID: c.ID})
	res := r.exec(ctx, c)
	turn := Turn{Replay: replay, ReplayGroup: group, Role: llm.RoleAssistant, Text: text, CallID: c.ID, Code: c.Code, Output: res.Output, Err: res.Err, Time: nowUTC()}
	if res.Err == "" && res.Value != "" {
		turn.Output += res.Value
	}
	s.Append(turn)
	r.emit(s, Ev{Kind: EvJSEnd, CallID: c.ID, Output: turn.Output, Err: res.Err})
	return ctx.Err()
}

// exec runs one call. With ModelTimeout the call's own timeout wins; a model
// that names none gets the default, and 0 or less means no limit.
func (r *Runner) exec(ctx context.Context, c call) jsvm.Result {
	if !r.D.ModelTimeout {
		return r.D.VM.Exec(ctx, c.Code)
	}
	d := defaultModelTimeout
	if c.Timeout != nil {
		d = time.Duration(*c.Timeout * float64(time.Second))
		if d < 0 {
			d = 0
		}
	}
	return r.D.VM.ExecTimeout(ctx, c.Code, d)
}

const defaultModelTimeout = 30 * time.Second
