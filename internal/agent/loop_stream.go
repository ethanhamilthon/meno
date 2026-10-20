package agent

import (
	"context"
	"errors"

	"meno/internal/llm"
)

func (r *Runner) streamAttempt(ctx context.Context, s *Session) ([]call, llm.Msg, error) {
	var calls []call
	replay := llm.Msg{Role: llm.RoleAssistant}
	var streamErr error
	req := llm.Request{
		Provider: r.D.Provider, Model: r.D.Model, Effort: r.D.Effort,
		System: r.D.System, Msgs: s.ToMsgs(), Tools: true, ToolDescription: r.D.ToolDesc, ToolName: r.D.ToolName,
	}
	r.sent = req.Msgs
	r.emit(s, Ev{Kind: EvRequest, Req: &req})
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
			c := call{ID: id, Code: e.Code, Title: llm.ToolTitle(e.Arguments)}
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

// calibrate compares the estimate of the request just answered with the input
// tokens the provider reported, so the next estimate is closer.
func (r *Runner) calibrate(s *Session, u llm.Usage) {
	if u.In <= 0 || r.D.AutoCompact.Window <= 0 {
		return
	}
	// The request just answered is the one built for it; building the messages
	// again would copy the whole history a second time.
	msgs := r.sent
	if msgs == nil {
		msgs = s.ToMsgs()
	}
	raw := estimateRequest(r.D.System, msgs, 0)
	if raw <= 0 {
		return
	}
	r.calibration = min(max(float64(u.In)/float64(raw), 0.5), 2.5)
}
