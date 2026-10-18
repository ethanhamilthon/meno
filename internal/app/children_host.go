package app

import (
	"errors"
	"fmt"
	"meno/internal/agent"
	"meno/internal/jsvm"
	"strings"
	"time"
)

// parentAgents implements jsvm.AgentsHost for a main session.
type parentAgents struct {
	a   *App
	m   *childMgr
	sid string
}

// childView is a child as the parent sees it.
func childView(r agent.ChildRecord) map[string]any {
	var result any
	if r.Result != nil {
		result = *r.Result
	}
	v := map[string]any{
		"id": r.ID, "status": r.Status, "task": eventSnippet(r.Task, 300), "generation": r.Gen,
		"result": result, "error": r.Error, "created": r.Created.Format(time.RFC3339),
		"usage": usageView(r),
	}
	if r.BlockReason != "" {
		v["blockReason"] = r.BlockReason
	}
	return v
}

// usageView keeps unknown prices apart from zero.
func usageView(r agent.ChildRecord) map[string]any {
	var cost any
	if !r.Unpriced || r.Cost > 0 {
		cost = r.Cost
	}
	return map[string]any{"in": r.Usage.In, "out": r.Usage.Out, "cached": r.Usage.Cached, "cacheWrite": r.Usage.CacheWrite, "cost": cost, "costIsLowerBound": r.Unpriced}
}

// snapshot is the record of a child, live when it runs in this process.
func (p parentAgents) snapshot(id string) (agent.ChildRecord, *childRun, error) {
	c, err := p.m.run(p.sid, id)
	if err != nil {
		return agent.ChildRecord{}, nil, err
	}
	if c == nil {
		rec, err := p.m.store.Get(p.sid, id)
		return rec, nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rec, c, nil
}

func (p parentAgents) List(jsvm.TaskCall) (any, error) {
	recs, err := p.m.store.List(p.sid, false)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(recs))
	for i, r := range recs {
		if live, _, e := p.snapshot(r.ID); e == nil {
			r = live
		}
		out[i] = childView(r)
	}
	return out, nil
}

func (p parentAgents) Status(_ jsvm.TaskCall, id string) (any, error) {
	r, _, err := p.snapshot(id)
	if err != nil {
		return nil, err
	}
	return childView(r), nil
}

func (p parentAgents) Result(_ jsvm.TaskCall, id string) (any, error) {
	r, _, err := p.snapshot(id)
	if err != nil {
		return nil, err
	}
	v := childView(r)
	return map[string]any{"id": r.ID, "status": r.Status, "result": v["result"], "error": r.Error, "usage": v["usage"]}, nil
}

func (p parentAgents) Cancel(_ jsvm.TaskCall, id string) (any, error) {
	if _, c, err := p.snapshot(id); err != nil {
		return nil, err
	} else if c != nil {
		p.m.cancel(c)
	}
	r, _, _ := p.snapshot(id)
	return childView(r), nil
}

func (p parentAgents) Wait(tc jsvm.TaskCall, id string, timeout time.Duration) (any, error) {
	r, c, err := p.snapshot(id)
	if err != nil {
		return nil, err
	}
	timedOut := false
	if c != nil && (r.Status == agent.ChildRunning || r.Status == agent.ChildStarting) {
		c.mu.Lock()
		idle := c.idle
		c.waiting++
		c.mu.Unlock()
		defer func() {
			c.mu.Lock()
			c.waiting--
			c.mu.Unlock()
		}()
		var expire <-chan time.Time
		if timeout > 0 {
			t := time.NewTimer(timeout)
			defer t.Stop()
			expire = t.C
		}
		select {
		case <-idle:
		case <-expire:
			timedOut = true
		case <-tc.Ctx.Done():
			return nil, tc.Ctx.Err()
		}
		r, _, _ = p.snapshot(id)
	}
	v := childView(r)
	v["waitTimedOut"] = timedOut
	if !timedOut && !r.Working() || r.Status == agent.ChildBlocked {
		if p.a.evStore != nil { // the model has seen this outcome: no second notice
			for _, kind := range []string{"completed", "failed", "blocked"} {
				_ = p.a.evStore.Ack(p.sid, childEventID(id, kind, r.Gen))
			}
		}
	}
	return v, nil
}

// requestID is the root request of the calling main session.
func (p parentAgents) requestID() string {
	var o string
	p.a.onUI(func() {
		if rt := p.a.rts[p.sid]; rt != nil {
			o = rt.RequestID
		}
	})
	return o
}

func (p parentAgents) Send(_ jsvm.TaskCall, id, text string) (any, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("agents.send: text is empty")
	}
	r, c, err := p.snapshot(id)
	if err != nil {
		return nil, err
	}
	if r.Status == agent.ChildCancelled || r.Status == agent.ChildInterrupted {
		return nil, fmt.Errorf("agent %s is %s and cannot be continued; start a new agent", id, r.Status)
	}
	if c == nil {
		if c, err = p.m.revive(r); err != nil {
			return nil, err
		}
	}
	return p.m.deliver(c, p.requestID(), agent.SourceParent, text)
}

func (p parentAgents) Report(jsvm.TaskCall, string) error {
	return errors.New("agents.report exists only for subagents")
}
func (p parentAgents) Block(jsvm.TaskCall, string) error {
	return errors.New("agents.block exists only for subagents")
}

// deliver sends text to a child. A running child gets it as steering and keeps
// its owner; a finished or blocked one starts a new execution generation under
// owner.
func (m *childMgr) deliver(c *childRun, owner, source, text string) (any, error) {
	c.mu.Lock()
	rec := c.rec
	c.mu.Unlock()
	id := agent.NewID()
	ev := agent.Event{ID: "msg-" + id, Session: rec.ID, Source: source, Kind: "message", Owner: rec.Owner, Text: text}
	if rec.Status == agent.ChildRunning || rec.Status == agent.ChildStarting {
		if _, err := m.a.evStore.Enqueue(ev); err != nil {
			return nil, err
		}
		return map[string]any{"accepted": true, "id": rec.ID}, nil
	}
	ev.Owner = owner
	if _, err := m.a.evStore.Enqueue(ev); err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.rec.Gen++
	c.rec.Owner = owner
	c.mu.Unlock()
	m.start(c)
	return map[string]any{"accepted": true, "id": rec.ID}, nil
}
