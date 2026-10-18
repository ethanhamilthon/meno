package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"meno/internal/llm"
)

type Runner struct {
	D Deps
	// cur counts the run in progress; Continue is not reentrant.
	cur *runCount
	// calibration scales estimates by what the provider reported last time.
	calibration    float64
	lastCompactLen int
	sent           []llm.Msg // messages of the request being answered
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
		if err := r.maybeCompact(ctx, s); err != nil {
			if errors.Is(err, context.Canceled) {
				return r.abort(s)
			}
			r.emit(s, Ev{Kind: EvError, Err: err.Error()})
			return err
		}
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
		if r.D.Halt != nil && r.D.Halt(batchFailed(s, len(calls))) {
			return ErrHalted
		}
	}
}

// ErrHalted is returned by Continue when Deps.Halt stopped the run.
var ErrHalted = errors.New("halted")

// batchFailed reports whether any of the last n tool results holds an error.
func batchFailed(s *Session, n int) bool {
	turns := s.Snapshot()
	for i := max(0, len(turns)-n); i < len(turns); i++ {
		if turns[i].CallID != "" && turns[i].Err != "" {
			return true
		}
	}
	return false
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
	case err == nil, errors.Is(err, ErrHalted):
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
	Title   string
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
