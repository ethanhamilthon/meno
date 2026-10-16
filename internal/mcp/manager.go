package mcp

import (
	"context"
	"sort"
	"sync"
)

type Manager struct {
	mu      sync.Mutex
	clients map[string]*Client
	errs    map[string]string
}

func NewManager() *Manager {
	return &Manager{clients: map[string]*Client{}, errs: map[string]string{}}
}

func (m *Manager) Start(ctx context.Context, s Server) error {
	m.Stop(s.Name)
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
