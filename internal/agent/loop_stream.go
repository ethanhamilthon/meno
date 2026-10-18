package agent

import (
	"context"
	"errors"
	"time"

	"meno/internal/jsvm"
	"meno/internal/llm"
)

func (r *Runner) streamAttempt(ctx context.Context, s *Session) ([]call, llm.Msg, error) {
	var calls []call
	replay := llm.Msg{Role: llm.RoleAssistant}
	var streamErr error
	req := llm.Request{
		Provider: r.D.Provider, Model: r.D.Model, Effort: r.D.Effort,
		System: r.D.System, Msgs: s.ToMsgs(), Tools: true, ModelTimeout: r.D.ModelTimeout,
	}
	fp := llm.FingerprintOf(req)
	r.emit(s, Ev{Kind: EvRequest, Fp: &fp})
	if r.D.DebugPayload != nil {
		r.D.DebugPayload(req)
	}
	err := r.D.Client.Stream(ctx, req, func(e llm.Event) {
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
			r.calibrate(s, e.Usage)
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
	limit := r.limit(c)
	r.emit(s, Ev{Kind: EvJSStart, Code: c.Code, CallID: c.ID, Limit: limit})
	res := r.exec(ctx, c)
	turn := Turn{Replay: replay, ReplayGroup: group, Role: llm.RoleAssistant, Text: text, CallID: c.ID, Code: c.Code, Output: res.Output, Err: res.Err, Time: nowUTC()}
	if res.Elapsed > 0 {
		turn.ElapsedMs = max(1, res.Elapsed.Milliseconds())
		turn.LimitMs = limit.Milliseconds()
		turn.TimedOut = res.TimedOut
		if r.cur != nil {
			r.cur.jsCalls++
			r.cur.jsTime += res.Elapsed
		}
	}
	if res.Err == "" && res.Value != "" {
		turn.Output += res.Value
	}
	s.Append(turn)
	r.emit(s, Ev{Kind: EvJSEnd, CallID: c.ID, Output: turn.Output, Err: res.Err, Elapsed: res.Elapsed, TimedOut: res.TimedOut})
	return ctx.Err()
}

// exec runs one call. With ModelTimeout the call's own timeout wins; a model
// that names none gets the default, and 0 or less means no limit.
func (r *Runner) exec(ctx context.Context, c call) jsvm.Result {
	if !r.D.ModelTimeout {
		return r.D.VM.Exec(ctx, c.Code)
	}
	return r.D.VM.ExecTimeout(ctx, c.Code, r.limit(c))
}

// limit is the timeout the call runs under (0 = none), as exec applies it.
func (r *Runner) limit(c call) time.Duration {
	if !r.D.ModelTimeout {
		if r.D.VM == nil {
			return 0
		}
		return r.D.VM.Timeout()
	}
	d := defaultModelTimeout
	if c.Timeout != nil {
		d = time.Duration(*c.Timeout * float64(time.Second))
		if d < 0 {
			d = 0
		}
	}
	return d
}

const defaultModelTimeout = 30 * time.Second

// calibrate compares the estimate of the request just answered with the input
// tokens the provider reported, so the next estimate is closer.
func (r *Runner) calibrate(s *Session, u llm.Usage) {
	if u.In <= 0 || r.D.AutoCompact.Window <= 0 {
		return
	}
	raw := estimateRequest(r.D.System, s.ToMsgs(), 0)
	if raw <= 0 {
		return
	}
	r.calibration = min(max(float64(u.In)/float64(raw), 0.5), 2.5)
}
