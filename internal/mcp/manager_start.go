package mcp

import (
	"context"
	"errors"
	"meno/internal/safe"
)

func (m *Manager) prepareStart(ctx context.Context, s Server) (func() error, error) {
	if err := ValidateServer(s); err != nil {
		m.setErr(s.Name, err.Error())
		return nil, err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, errors.New("mcp: manager closed")
	}
	old := m.clients[s.Name]
	delete(m.clients, s.Name)
	if cancel := m.cancels[s.Name]; cancel != nil {
		cancel()
	}
	ctx, cancel := context.WithCancel(ctx)
	m.cancels[s.Name] = cancel
	m.generation[s.Name]++
	generation := m.generation[s.Name]
	m.starting[s.Name]++
	delete(m.errs, s.Name)
	m.mu.Unlock()
	if old != nil {
		old.Close()
	}
	m.changed(s.Name)
	return func() error {
		c, err := Start(ctx, s)
		m.mu.Lock()
		current := !m.closed && m.generation[s.Name] == generation && ctx.Err() == nil
		if current {
			if err == nil {
				m.clients[s.Name] = c
				delete(m.errs, s.Name)
			} else {
				m.errs[s.Name] = err.Error()
			}
		}
		m.mu.Unlock()
		m.setStarting(s.Name, false)
		if !current {
			cancel()
			if c != nil {
				c.Close()
			}
			return context.Canceled
		}
		m.changed(s.Name)
		if err == nil {
			go func() { <-c.done; m.changed(s.Name) }()
		}
		return err
	}, nil
}

func (m *Manager) Start(ctx context.Context, s Server) error {
	run, err := m.prepareStart(ctx, s)
	if err != nil {
		return err
	}
	return run()
}

// StartAsync registers the attempt before returning to the caller.
func (m *Manager) StartAsync(ctx context.Context, s Server) {
	run, err := m.prepareStart(ctx, s)
	if err != nil {
		return
	}
	go func() { defer safe.Recover(nil); _ = run() }()
}
