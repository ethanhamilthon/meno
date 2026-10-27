package tui

import (
	"context"
	"testing"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/render"
)

// store gives tests the same views of the daemon's data the old store did.
type testStore struct {
	a *App
}

func (a *App) storeForTest() testStore {
	a.running.Wait() // let the background saves reach the daemon
	return testStore{a: a}
}

func (s testStore) LoadProviders() ([]llm.Provider, error) {
	r, err := s.a.fetchConfigErr()
	return r.providers, err
}

func (s testStore) LoadSkills() ([]config.Skill, error) {
	var out []config.Skill
	err := s.a.rpc(func(ctx context.Context, c *client.Client) error {
		r, e := c.Skills(ctx, s.a.st.Cwd)
		for _, k := range r.Skills {
			out = append(out, config.Skill{Name: k.Name, Scope: k.Scope, Desc: k.Desc, Path: k.Path, Enabled: k.Enabled, Auto: k.Auto})
		}
		return e
	})
	return out, err
}

func (a *App) fetchConfigErr() (settingsLoaded, error) {
	m := a.fetchConfig()
	return m, m.err
}

// saveSession is a no-op: the daemon saves as it goes.
func (a *App) saveSession() {}

// Snapshot is the turns of the session, for the old tests.
func (s *sessView) Snapshot() []render.Turn { return s.turns }

var _ = (*testing.T)(nil)
