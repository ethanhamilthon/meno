package mcp

type Status struct {
	Running  bool
	Starting bool
	Error    string
}

func (m *Manager) Snapshot(name string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.clients[name]
	err := m.errs[name]
	if c != nil && !c.Alive() && err == "" {
		err = "MCP server disconnected"
	}
	return Status{Running: c != nil && c.Alive(), Starting: m.starting[name] > 0, Error: err}
}

func (m *Manager) SetOnChange(fn func(string)) {
	m.mu.Lock()
	m.onChange = fn
	m.mu.Unlock()
}

func (m *Manager) changed(name string) {
	m.mu.Lock()
	fn := m.onChange
	m.mu.Unlock()
	if fn != nil {
		fn(name)
	}
}
