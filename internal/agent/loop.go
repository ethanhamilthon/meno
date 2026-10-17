package agent

import (
	"context"
	"errors"
	"strings"
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
	// Attempt, Max and Delay describe an EvRetry: attempt number Attempt of Max
	// starts after Delay; Err is why the previous one failed.
	Attempt, Max int
	Delay        time.Duration
	// Limit is the timeout of the js call an EvJSStart announces (0 = none);
	// Elapsed and TimedOut report how an EvJSEnd call went.
	Limit    time.Duration
	Elapsed  time.Duration
	TimedOut bool
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
	// EvInjected: queued user messages were added to the session.
	EvInjected = "injected"
	// EvRetry: the request failed and will be sent again; whatever it streamed
	// so far is void.
	EvRetry = "retry"
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
	// Retry is the policy for failed model requests (zero value: defaults).
	Retry RetryPolicy
	// Queue, when set, delivers messages the user wrote during the turn; they
	// join the conversation between steps (never between a call and its result).
	Queue *Queue
	// Cost prices the usage of one request; ok is false when the model has no
	// price. Optional: without it a run's summary carries no cost.
	Cost func(llm.Usage) (cost float64, ok bool)
	Emit func(Ev)
}

type Runner struct {
	D Deps
	// cur counts the run in progress; Continue is not reentrant.
	cur *runCount
}

// runCount is what a run has done so far.
type runCount struct {
	start    time.Time
	turns    int
	jsCalls  int
	jsTime   time.Duration
	usage    llm.Usage
	cost     float64
	unpriced bool
}

func (r *Runner) Turn(ctx context.Context, s *Session, userText string) error {
	s.Append(Turn{Role: llm.RoleUser, Text: userText})
	return r.Continue(ctx, s)
}

// Continue runs the agent until the model answers without a tool call. The last
// turn of the session gets the stats of the run (Turn.Run).
func (r *Runner) Continue(ctx context.Context, s *Session) error {
	r.cur = &runCount{start: time.Now()}
	err := r.loop(ctx, s)
	r.finish(s, err)
	return err
}

func (r *Runner) loop(ctx context.Context, s *Session) error {
	// No step limit: the loop ends when the model stops calling tools, on error,
	// or when ctx is canceled (ctrl+c).
	for {
		if err := ctx.Err(); err != nil {
			return r.abort(s)
		}
		r.inject(s)
		calls, replay, err := r.streamOnce(ctx, s)
		if err == nil {
			r.cur.turns++
		}
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
			if r.D.Queue != nil && r.D.Queue.Len() > 0 {
				continue // the user spoke meanwhile: answer without ending the turn
			}
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

// inject adds the queued user messages to the session as one turn.
func (r *Runner) inject(s *Session) {
	if r.D.Queue == nil {
		return
	}
	items := r.D.Queue.Take()
	if len(items) == 0 {
		return
	}
	var viewed, written []Message
	for _, m := range items {
		if m.Viewed {
			viewed = append(viewed, m)
		} else {
			written = append(written, m)
		}
	}
	if len(viewed) > 0 {
		m := Combine(viewed)
		s.Append(Turn{Role: llm.RoleUser, Kind: KindView, Text: m.Text, Attachments: m.Attachments})
	}
	if len(written) > 0 {
		m := Combine(written)
		s.Append(Turn{Role: llm.RoleUser, Text: m.Text, Attachments: m.Attachments})
	}
	r.emit(s, Ev{Kind: EvInjected})
}

func (r *Runner) emit(s *Session, e Ev) {
	e.Turns = s.Len()
	if e.Kind == EvUsage && r.cur != nil {
		r.cur.usage.In += e.Usage.In
		r.cur.usage.Out += e.Usage.Out
		r.cur.usage.Cached += e.Usage.Cached
		r.cur.usage.CacheWrite += e.Usage.CacheWrite
		if r.D.Cost != nil {
			if c, ok := r.D.Cost(e.Usage); ok {
				r.cur.cost += c
			} else {
				r.cur.unpriced = true
			}
		}
	}
	r.D.Emit(e)
}

// finish attaches the stats of the finished run to the session.
func (r *Runner) finish(s *Session, err error) {
	c := r.cur
	if c == nil {
		return
	}
	st := RunStats{
		Status: RunOK, ElapsedMs: time.Since(c.start).Milliseconds(),
		Turns: c.turns, JSCalls: c.jsCalls, JSMs: c.jsTime.Milliseconds(), Usage: c.usage,
	}
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled):
		st.Status = RunStop
	default:
		st.Status, st.Err = RunFail, briefErr(err.Error())
	}
	if r.D.Cost != nil && !c.unpriced && c.turns > 0 {
		cost := c.cost
		st.Cost = &cost
	}
	s.SetRun(st)
	r.cur = nil
}

// briefErr squeezes an error into one line of at most 300 characters.
func briefErr(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 300 {
		s = string(r[:299]) + "…"
	}
	return s
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

// streamOnce asks the model once (with retries) and returns the calls it made.
func (r *Runner) streamOnce(ctx context.Context, s *Session) (calls []call, replay llm.Msg, err error) {
	err = r.withRetry(ctx, s, func() error {
		calls, replay, err = r.streamAttempt(ctx, s)
		return err
	})
	return calls, replay, err
}

func (r *Runner) streamAttempt(ctx context.Context, s *Session) ([]call, llm.Msg, error) {
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
