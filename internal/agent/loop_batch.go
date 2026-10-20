package agent

import (
	"context"
	"fmt"
	"time"

	"meno/internal/llm"
)

// batchCall is one call of a model response while it runs.
type batchCall struct {
	call
	run     RunningCall
	started time.Time
	elapsed time.Duration
	res     ToolResult
	taskID  string
	done    bool
}

// runBatch starts every call of one model response at once, after the response
// is complete, and waits for them. Results are stored in call order with one
// replay group, whatever order the calls finish in. A failing call never stops
// its siblings. A message from the user promotes the calls still running to
// background tasks; Stop (ctx) cancels them. Either way every announced call
// gets exactly one result.
func (r *Runner) runBatch(ctx context.Context, s *Session, text string, calls []call, replay *llm.Msg) error {
	bs := make([]*batchCall, len(calls))
	fin := make(chan int, len(calls))
	for i, c := range calls {
		if c.ID == "" {
			c.ID = NewID()
		}
		b := &batchCall{call: c, started: time.Now()}
		bs[i] = b
		r.emit(s, Ev{Kind: EvJSStart, Code: c.Code, Title: c.Title, CallID: c.ID})
		if r.D.Tool == nil {
			b.res, b.done = ToolResult{Err: "tool runner is not configured"}, true
			fin <- i
			continue
		}
		run, err := r.D.Tool.Start(c.ID, c.Code)
		if err != nil {
			b.res, b.done = ToolResult{Err: err.Error()}, true
			fin <- i
			continue
		}
		b.run = run
		go func(i int, run RunningCall) { <-run.Done(); fin <- i }(i, run)
	}
	pending := len(bs)
	stopped := false
	var human <-chan struct{}
	if r.D.Queue != nil {
		human = r.D.Queue.Human()
	}
	promote := func() {
		for _, b := range bs {
			if b.done || b.run == nil {
				continue
			}
			if id, ok := b.run.Promote(); ok {
				b.taskID, b.done = id, true
				b.elapsed = time.Since(b.started)
				b.res = ToolResult{Output: promotedOutput(id)}
				pending--
				r.emit(s, Ev{Kind: EvJSEnd, CallID: b.ID, Output: b.res.Output, Elapsed: b.elapsed, TaskID: id})
			}
		}
	}
	if r.D.Queue != nil && r.D.Queue.Len() > 0 {
		promote()
	}
	for pending > 0 {
		select {
		case i := <-fin:
			b := bs[i]
			if b.done && b.taskID != "" {
				continue // promoted while finishing: the task owns the outcome
			}
			if !b.done || b.run == nil {
				if b.run != nil {
					b.res = b.run.Result()
				}
				b.done = true
			}
			b.elapsed = time.Since(b.started)
			pending--
			r.emit(s, Ev{Kind: EvJSEnd, CallID: b.ID, Output: b.res.Output, Err: b.res.Err, Elapsed: b.elapsed})
		case <-human:
			if r.D.Queue.Len() > 0 {
				promote()
			}
		case <-ctx.Done():
			if !stopped {
				stopped = true
				for _, b := range bs {
					if !b.done && b.run != nil {
						b.run.Cancel()
					}
				}
			}
			human = nil
			ctx = context.Background() // wait for the cancelled calls to end
		}
	}
	group := NewID()
	for i, b := range bs {
		turn := Turn{ReplayGroup: group, Role: llm.RoleAssistant, CallID: b.ID, Code: b.Code, Title: b.Title, Output: b.res.Output, Err: b.res.Err, Time: nowUTC()}
		if i == 0 {
			turn.Text, turn.Replay = text, replay
		}
		if b.elapsed > 0 {
			turn.ElapsedMs = max(1, b.elapsed.Milliseconds())
			if r.cur != nil {
				r.cur.jsCalls++
				r.cur.jsTime += b.elapsed
			}
		}
		s.Append(turn)
	}
	if stopped {
		return context.Canceled
	}
	return nil
}

// promotedOutput is the one result of a call that became a background task.
func promotedOutput(id string) string {
	return fmt.Sprintf("The user sent a message, so this command was moved to background task %s. It is still running and its output is not part of this result.\n"+
		"Inspect it with: meno tasks status %s | meno tasks logs %s | meno tasks wait %s | meno tasks cancel %s\n"+
		"A notice arrives when it finishes.", id, id, id, id, id)
}
