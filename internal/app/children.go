package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"meno/internal/agent"
	"meno/internal/jsvm"
	"meno/internal/llm"
)

// childRun is a subagent that exists in this run of meno: its session, its
// runtime and the goroutine that drives its agent loop. The loop is the same
// agent.Runner as the main one, with its own queue, events and VM.
type childRun struct {
	mu     sync.Mutex
	parent string // immutable copy of rec.Parent
	rec    agent.ChildRecord
	sess   *agent.Session
	vm     *jsvm.VM
	queue  agent.Queue
	cancel context.CancelFunc
	idle   chan struct{} // closed when the current execution generation ends

	deps    agent.Deps
	price   func(llm.Usage) (float64, bool)
	waiting int    // parents blocked in agents.wait on this child
	block   string // a block requested in the current tool batch
	failed  bool
}

// childMgr runs the subagents of every main session of this process.
type childMgr struct {
	a       *App
	store   *agent.ChildStore
	mu      sync.Mutex
	runs    map[string]*childRun
	wg      sync.WaitGroup
	closing bool
}

type childChanged struct{ parent string }

func newChildMgr(a *App) *childMgr {
	store, err := agent.OpenChildren(a.paths.Home)
	if err != nil {
		return nil
	}
	_ = store.MarkInterrupted()
	return &childMgr{a: a, store: store, runs: map[string]*childRun{}}
}

func eventSnippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = s[:n] + "… [cut; the full text is in agents.result]"
	}
	return s
}

// finish records the end of an execution generation and tells the parent.
func (m *childMgr) finish(c *childRun, runErr error) {
	c.mu.Lock()
	rec := c.rec
	switch {
	case rec.Status == agent.ChildCancelled:
	case errors.Is(runErr, agent.ErrHalted):
		rec.Status, rec.BlockReason = agent.ChildBlocked, c.block
	case runErr != nil && !errors.Is(runErr, context.Canceled):
		rec.Status, rec.Error = agent.ChildFailed, runErr.Error()
	case errors.Is(runErr, context.Canceled):
		rec.Status = agent.ChildCancelled
	default:
		rec.Status = agent.ChildCompleted
		res := lastAssistantText(c.sess)
		rec.Result = &res
	}
	c.block, c.failed = "", false
	rec.Ended = time.Now().UTC()
	c.rec = rec
	idle := c.idle
	waited := c.waiting > 0
	c.mu.Unlock()
	_ = agent.SaveSession(m.a.paths.Home, c.sess)
	_ = m.store.Save(rec)
	var text, kind string
	switch rec.Status {
	case agent.ChildCompleted:
		kind, text = "completed", "Agent "+rec.ID+" completed. Result:\n"+eventSnippet(*rec.Result, 3000)
	case agent.ChildFailed:
		kind, text = "failed", "Agent "+rec.ID+" failed: "+eventSnippet(rec.Error, 1000)
	case agent.ChildBlocked:
		kind, text = "blocked", "Agent "+rec.ID+" is blocked and waits for your reply (agents.send): "+eventSnippet(rec.BlockReason, 1500)
	}
	if kind != "" {
		m.a.enqueueEventAcked(agent.Event{ID: childEventID(rec.ID, kind, rec.Gen), Session: rec.Parent, Source: agent.SourceChild, From: rec.ID,
			Kind: kind, Owner: rec.Owner, Text: text, Refs: []string{"child:" + rec.ID}}, waited)
	}
	close(idle)
	m.a.postEvent(childChanged{rec.Parent})
}

func childEventID(id, kind string, gen int) string {
	return fmt.Sprintf("agent-%s-%s-g%d", id, kind, gen)
}

func lastAssistantText(s *agent.Session) string {
	turns := s.Snapshot()
	for i := len(turns) - 1; i >= 0; i-- {
		if t := turns[i]; t.Role == llm.RoleAssistant && t.Code == "" && strings.TrimSpace(t.Text) != "" {
			return strings.TrimSpace(t.Text)
		}
	}
	return ""
}

// start runs one execution generation of c in the background.
func (m *childMgr) start(c *childRun) {
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.cancel, c.idle = cancel, make(chan struct{})
	c.rec.Status = agent.ChildRunning
	c.rec.Ended = time.Time{}
	c.rec.Error, c.rec.BlockReason = "", ""
	rec := c.rec
	deps := c.deps
	c.mu.Unlock()
	_ = m.store.Save(rec)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer cancel()
		err := (&agent.Runner{D: deps}).Continue(ctx, c.sess)
		m.finish(c, err)
	}()
	m.a.postEvent(childChanged{rec.Parent})
}

// onEmit accounts the usage of a child's requests; it is the child's Emit.
func (c *childRun) onEmit(e agent.Ev) {
	if e.Kind != agent.EvUsage {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	u := &c.rec.Usage
	u.In, u.Out, u.Cached, u.CacheWrite = u.In+e.Usage.In, u.Out+e.Usage.Out, u.Cached+e.Usage.Cached, u.CacheWrite+e.Usage.CacheWrite
	if c.price != nil {
		if cost, ok := c.price(e.Usage); ok {
			c.rec.Cost += cost
			return
		}
	}
	c.rec.Unpriced = true
}

// halt ends the run after a tool batch that asked to block, unless a call of
// the batch failed: the failure wins and the child goes on.
func (c *childRun) halt(failed bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.block == "" {
		return false
	}
	if failed {
		c.block = ""
		return false
	}
	return true
}

// run returns the live child with this ID of the given parent.
func (m *childMgr) run(parent, id string) (*childRun, error) {
	m.mu.Lock()
	c := m.runs[id]
	m.mu.Unlock()
	if c != nil && c.parent == parent {
		return c, nil
	}
	if _, err := m.store.Get(parent, id); err != nil {
		return nil, err
	}
	return nil, nil // known, but not in this run of meno
}

// Shutdown cancels every child and joins their goroutines.
func (m *childMgr) Shutdown() {
	m.mu.Lock()
	m.closing = true
	var all []*childRun
	for _, c := range m.runs {
		all = append(all, c)
	}
	m.mu.Unlock()
	for _, c := range all {
		c.mu.Lock()
		if c.cancel != nil && c.rec.Working() {
			c.rec.Status = agent.ChildCancelled
			c.cancel()
		}
		c.mu.Unlock()
	}
	m.wg.Wait()
}

// CancelOwner cancels the children a root request owns (:stop).
func (m *childMgr) CancelOwner(parent, owner string) {
	m.mu.Lock()
	var hit []*childRun
	for _, c := range m.runs {
		c.mu.Lock()
		if c.parent == parent && c.rec.Owner == owner && c.rec.Working() {
			hit = append(hit, c)
		}
		c.mu.Unlock()
	}
	m.mu.Unlock()
	for _, c := range hit {
		m.cancel(c)
	}
}

func (m *childMgr) cancel(c *childRun) {
	c.mu.Lock()
	working := c.rec.Working()
	if working {
		c.rec.Status = agent.ChildCancelled
	}
	cancel, idle := c.cancel, c.idle
	c.mu.Unlock()
	if working && cancel != nil {
		cancel()
		select {
		case <-idle:
		case <-time.After(10 * time.Second):
		}
	}
	c.mu.Lock()
	rec := c.rec
	c.mu.Unlock()
	_ = m.store.Save(rec)
	m.a.postEvent(childChanged{rec.Parent})
}
