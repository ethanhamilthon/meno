package agent

import (
	"context"
	"math/rand/v2"
	"time"

	"meno/internal/llm"
)

// RetryPolicy controls how a failed model request is repeated. The zero value
// is the default: 5 attempts in total, 1s, 2s, 4s, 8s between them (at most
// 20s), jittered by ±20%; a Retry-After from the provider wins but is capped.
type RetryPolicy struct {
	Attempts      int           // total attempts including the first
	Base, Cap     time.Duration // first delay and its ceiling
	MaxRetryAfter time.Duration
}

func (p RetryPolicy) norm() RetryPolicy {
	if p.Attempts <= 0 {
		p.Attempts = 5
	}
	if p.Base <= 0 {
		p.Base = time.Second
	}
	if p.Cap <= 0 {
		p.Cap = 20 * time.Second
	}
	if p.MaxRetryAfter <= 0 {
		p.MaxRetryAfter = 60 * time.Second
	}
	return p
}

// delay is the wait before attempt number next (2 is the first retry).
func (p RetryPolicy) delay(next int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return min(retryAfter, p.MaxRetryAfter)
	}
	d := p.Base
	for i := 2; i < next && d < p.Cap; i++ {
		d *= 2
	}
	d = min(d, p.Cap)
	return time.Duration(float64(d) * (0.8 + 0.4*rand.Float64()))
}

// sleep waits for d or until ctx ends.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// withRetry runs one model request, repeating it from scratch while the error
// is retryable. attempt discards whatever a failed try produced, so a request
// that dropped mid-stream starts over cleanly; tool calls only run after a
// request has completed, so nothing is executed twice.
func (r *Runner) withRetry(ctx context.Context, s *Session, attempt func() error) error {
	p := r.D.Retry.norm()
	for n := 1; ; n++ {
		err := attempt()
		if err == nil || ctx.Err() != nil {
			return err
		}
		ok, after := llm.Retryable(err)
		if !ok || n >= p.Attempts {
			return err
		}
		d := p.delay(n+1, after)
		r.emit(s, Ev{Kind: EvRetry, Attempt: n + 1, Max: p.Attempts, Delay: d, Err: err.Error()})
		if serr := sleep(ctx, d); serr != nil {
			return serr
		}
	}
}
