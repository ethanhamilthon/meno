package app

import (
	"errors"
	"path/filepath"
	"time"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/jsvm"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/prompt"
)

// build gives a child its runtime: a VM with the ordinary tools but without
// askUser, tasks, nested agents, and the
// agent loop dependencies.
func (m *childMgr) build(c *childRun, pr llm.Provider, model, effort, system string, st config.Settings) error {
	a := m.a
	vm := jsvm.New(jsvm.Options{Root: c.sess.Cwd, Timeout: st.TimeoutDuration()})
	vm.SetMCP(mcpHost{a.mcp, mcp.ResultStore{Dir: filepath.Join(a.paths.Home, "mcp-results")}, c.sess.ID})
	vm.SetSessionHost(sessionHost{a: a, sid: c.sess.ID, sess: c.sess, child: true})
	vm.SetAgents(childAgents{m, c}, true)
	c.vm = vm
	c.deps = agent.Deps{
		Client: a.client(pr), Provider: pr, Model: model, Effort: effort, System: system, VM: vm,
		ModelTimeout: st.ModelTimeout(), Queue: &c.queue, Events: agent.StoreSource{Store: a.evStore, Session: c.sess.ID},
		Cost: c.price, Emit: c.onEmit, Halt: c.halt, Retry: a.providerRetry(pr),
	}
	return nil
}

// revive brings a child of an earlier run (or a finished one) back so that it
// can continue: its session is loaded and its runtime rebuilt.
func (m *childMgr) revive(rec agent.ChildRecord) (*childRun, error) {
	m.mu.Lock()
	if c := m.runs[rec.ID]; c != nil {
		m.mu.Unlock()
		return c, nil
	}
	m.mu.Unlock()
	sess, err := agent.LoadSession(m.a.paths.Home, rec.ID)
	if err != nil {
		return nil, err
	}
	sess.AttachDir = m.a.attachDir()
	var pr llm.Provider
	var st config.Settings
	var price func(llm.Usage) (float64, bool)
	var src worldSrc
	found := false
	m.a.onUI(func() {
		for _, cand := range m.a.st.Providers {
			if cand.Name == sess.Provider {
				pr, found = cand, true
			}
		}
		st = m.a.st.Settings
		price = m.a.pricer(pr, sess.Model)
		src = m.a.worldSrc()
		src.Paths.Cwd = sess.Cwd
		src.Model = modelSig{sess.Provider, sess.Model, sess.Effort}
	})
	if !found {
		return nil, errors.New("the provider of this agent is no longer configured")
	}
	if !dirExists(sess.Cwd) {
		return nil, errors.New("the working directory of this agent no longer exists")
	}
	w, _ := src.gather(contextBackground())
	w.Input.Subagent = true
	c := &childRun{sess: sess, rec: rec, price: price, parent: rec.Parent}
	if err := m.build(c, pr, sess.Model, sess.Effort, prompt.Build(w.Input), st); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.runs[rec.ID] = c
	m.mu.Unlock()
	return c, nil
}

// childAgents implements jsvm.AgentsHost for a child: only report and block.
type childAgents struct {
	m *childMgr
	c *childRun
}

var errChildAPI = errors.New("not available to a subagent: only agents.report and agents.block exist here")

func (childAgents) Spawn(jsvm.TaskCall, map[string]any) (any, error)       { return nil, errChildAPI }
func (childAgents) List(jsvm.TaskCall) (any, error)                        { return nil, errChildAPI }
func (childAgents) Status(jsvm.TaskCall, string) (any, error)              { return nil, errChildAPI }
func (childAgents) Send(jsvm.TaskCall, string, string) (any, error)        { return nil, errChildAPI }
func (childAgents) Wait(jsvm.TaskCall, string, time.Duration) (any, error) { return nil, errChildAPI }
func (childAgents) Result(jsvm.TaskCall, string) (any, error)              { return nil, errChildAPI }
func (childAgents) Cancel(jsvm.TaskCall, string) (any, error)              { return nil, errChildAPI }

// Report sends a bounded progress note to the parent through the steering queue.
func (h childAgents) Report(_ jsvm.TaskCall, text string) error {
	c := h.c
	c.mu.Lock()
	rec := c.rec
	c.mu.Unlock()
	h.m.a.enqueueEvent(agent.Event{ID: "report-" + agent.NewID(), Session: rec.Parent, Source: agent.SourceChild, From: rec.ID, Kind: "report",
		Owner: rec.Owner, Text: eventSnippet(text, 2000), Refs: []string{"child:" + rec.ID}})
	return nil
}

// Block asks to stop after the current tool batch and wait for the parent.
func (h childAgents) Block(_ jsvm.TaskCall, reason string) error {
	if reason == "" {
		return errors.New("agents.block: say what you are blocked on")
	}
	h.c.mu.Lock()
	h.c.block = reason
	h.c.mu.Unlock()
	return nil
}
