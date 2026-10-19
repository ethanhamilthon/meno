package agent

import (
	"context"
	"errors"
	"fmt"

	"meno/internal/llm"
)

// AutoCompact configures the safety net that compacts before a request would
// overflow the context window.
type AutoCompact struct {
	// Window is the context window of the model in tokens. 0 means unknown: the
	// net is off, because a window is never guessed.
	Window int
	// Percent of the window at which to compact (default 80).
	Percent int
	// Keep is the minimum recent messages that stay (default 8).
	Keep int
	// SummaryTokens is the target size of the summary (default 4000).
	SummaryTokens int
	// OutputReserve is the room left for the answer (default 8192 tokens).
	OutputReserve int
}

func (c AutoCompact) percent() int {
	if c.Percent <= 0 || c.Percent > 100 {
		return 80
	}
	return c.Percent
}

func (c AutoCompact) keep() int {
	if c.Keep <= 0 {
		return 8
	}
	return c.Keep
}

func (c AutoCompact) summary() int {
	if c.SummaryTokens <= 0 {
		return 4000
	}
	return c.SummaryTokens
}

func (c AutoCompact) reserve() int {
	if c.OutputReserve <= 0 {
		return 8192
	}
	return c.OutputReserve
}

// estimateRequest is a rough token count of everything a request carries: the
// system prompt, every message and image, with a fixed allowance for tool
// schemas. It is calibrated by the usage the provider reported for the last
// request (calibration), not trusted as exact accounting.
func estimateRequest(system string, msgs []llm.Msg, calibration float64) int {
	n := len(system) + 2000
	for _, m := range msgs {
		n += len(m.Text) + len(m.Code) + len(m.Thought)
		for _, tc := range m.ToolCalls {
			n += len(tc.Arguments) + len(tc.Name)
		}
		for _, r := range m.Responses {
			n += len(r)
		}
		n += 1200 * len(m.Images)
	}
	est := float64(n) / 4
	if calibration > 0 {
		est *= calibration
	}
	return int(est)
}

// ErrNoRoom is returned when the request cannot fit even after compaction.
var ErrNoRoom = errors.New("the context window is too small for this conversation even after compaction")

// maybeCompact runs before each model request. When the estimated request
// passes the threshold it summarizes the old part of the context. It refuses
// to compact the same unchanged context twice: that would loop.
func (r *Runner) maybeCompact(ctx context.Context, s *Session) error {
	ac := r.D.AutoCompact
	if ac.Window <= 0 {
		return nil
	}
	est := estimateRequest(r.D.System, s.ToMsgs(), r.calibration)
	limit := ac.Window * ac.percent() / 100
	if est < limit {
		r.lastCompactLen = -1
		return nil
	}
	if r.lastCompactLen == s.Len() {
		// Compacted this exact context already and it still does not fit.
		if est+ac.reserve() > ac.Window {
			return fmt.Errorf("%w (estimated %d tokens of %d)", ErrNoRoom, est, ac.Window)
		}
		return nil
	}
	if s.Len() <= ac.keep() {
		if est+ac.reserve() > ac.Window {
			return fmt.Errorf("%w (estimated %d tokens of %d, only %d messages)", ErrNoRoom, est, ac.Window, s.Len())
		}
		return nil
	}
	_, _, err := CompactWith(ctx, r.D.Client, r.D.Provider, r.D.Model, s, CompactOptions{Keep: ac.keep(), SummaryTokens: ac.summary(), System: r.D.System, Effort: r.D.Effort, ModelTimeout: r.D.ModelTimeout, ToolDesc: r.D.ToolDesc},
		func(u llm.Usage) { r.emit(s, Ev{Kind: EvUsage, Usage: u, Provider: r.D.Provider, Model: r.D.Model}) })
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// The old context is intact; say so and try the request anyway when it fits.
		r.emit(s, Ev{Kind: EvError, Err: "automatic compaction failed: " + err.Error()})
		if est+ac.reserve() > ac.Window {
			return fmt.Errorf("%w: %v", ErrNoRoom, err)
		}
		return nil
	}
	r.lastCompactLen = s.Len()
	r.emit(s, Ev{Kind: EvInjected})
	return nil
}
