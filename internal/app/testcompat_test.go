package app

import (
	"context"
	"testing"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/ui"
)

// store gives tests the same views of the daemon's data the old store did.
type testStore struct {
	a *App
	snippetStore
}

func (a *App) storeForTest() testStore {
	a.running.Wait() // let the background saves reach the daemon
	return testStore{a: a, snippetStore: snippetStore{a}}
}

func (s testStore) LoadSettings() (config.Settings, error) {
	var out config.Settings
	err := s.a.rpc(func(ctx context.Context, c *client.Client) error {
		r, e := c.Settings(ctx)
		out = settingsFromWire(r.Settings)
		return e
	})
	return out, err
}

func (s testStore) LoadProviders() ([]llm.Provider, error) {
	r, err := s.a.fetchConfigErr()
	return r.providers, err
}

func (s testStore) LoadSkills() ([]config.Skill, error) {
	r, err := s.a.fetchConfigErr()
	return r.skills, err
}

func (a *App) fetchConfigErr() (settingsLoaded, error) {
	m := a.fetchConfig()
	return m, m.err
}

// saveSession is a no-op: the daemon saves as it goes.
func (a *App) saveSession() {}

// Snapshot is the turns of the session, for the old tests.
func (s *sessView) Snapshot() []ui.Turn { return s.turns }

var _ = (*testing.T)(nil)
