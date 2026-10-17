package mcp

import (
	"context"
	"sort"
	"sync"
	"time"
)

type Manager struct {
	mu       sync.Mutex
	clients  map[string]*Client
	errs     map[string]string
	starting map[string]int // servers being started now (a count: BeginStart and Start both mark)
	cond     *sync.Cond
}

func NewManager() *Manager {
	m := &Manager{clients: map[string]*Client{}, errs: map[string]string{}, starting: map[string]int{}}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *Manager) Start(ctx context.Context, s Server) error {
	m.Stop(s.Name)
	m.setStarting(s.Name, true)
	defer m.setStarting(s.Name, false)
	if err := ValidateServer(s); err != nil {
		m.setErr(s.Name, err.Error())
		return err
	}
	c, err := Start(ctx, s)
	if err != nil {
		m.setErr(s.Name, err.Error())
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[s.Name] = c
	delete(m.errs, s.Name)
	return nil
}

func (m *Manager) setStarting(name string, on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if on {
		m.starting[name]++
		return
	}
	if m.starting[name] <= 1 {
		delete(m.starting, name)
	} else {
		m.starting[name]--
	}
	m.cond.Broadcast()
}

// BeginStart marks a server as starting before its goroutine has begun, so
// that WaitReady cannot miss it. Pair it with EndStart.
func (m *Manager) BeginStart(name string) { m.setStarting(name, true) }

// EndStart undoes BeginStart.
func (m *Manager) EndStart(name string) { m.setStarting(name, false) }

// WaitReady blocks until no server is starting, or until ctx ends or the timeout
// passes. It reports whether every server settled.
func (m *Manager) WaitReady(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	// A sync.Cond cannot be waited on with a timeout: wake it up from a timer.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTimer(timeout)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
		case <-stop:
			return
		}
		m.mu.Lock()
		m.cond.Broadcast()
		m.mu.Unlock()
	}()
	m.mu.Lock()
	defer m.mu.Unlock()
	for len(m.starting) > 0 {
		if ctx.Err() != nil || !time.Now().Before(deadline) {
			return false
		}
		m.cond.Wait()
	}
	return true
}

// Starting reports whether a server is being started right now.
func (m *Manager) Starting(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.starting[name] > 0
}

// ToolsOf lists the tools of a running server.
func (m *Manager) ToolsOf(name string) []Tool {
	c, ok := m.Client(name)
	if !ok {
		return nil
	}
	return c.Tools
}

func (m *Manager) setErr(name, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errs[name] = msg
}

func (m *Manager) Stop(name string) {
	m.mu.Lock()
	c := m.clients[name]
	delete(m.clients, name)
	m.mu.Unlock()
	if c != nil {
		c.Close()
	}
}

func (m *Manager) Running(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.clients[name]
	return c != nil && c.Alive()
}

func (m *Manager) Err(name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errs[name]
}

func (m *Manager) Client(name string) (*Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.clients[name]
	return c, ok && c.Alive()
}

func (m *Manager) Names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.clients))
	for n := range m.clients {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (m *Manager) ToolCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.clients {
		n += len(c.Tools)
	}
	return n
}

func (m *Manager) StopAll() {
	for _, n := range m.Names() {
		m.Stop(n)
	}
}
