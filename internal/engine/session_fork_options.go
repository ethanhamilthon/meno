package engine

import (
	"context"
	"os"
	"path/filepath"

	"meno/internal/agent"
	"meno/internal/api"
)

// forkChanges are the checked settings a fork gets instead of its source's.
type forkChanges struct {
	cwd, title string
	model      *modelChoice
	system     *[2]string // name, text
	noAsk      bool
}

// forkOverrides validates everything a fork request changes before any session
// is created, so a bad request leaves nothing behind.
func (e *Engine) forkOverrides(ctx context.Context, src *runState, q api.ForkRequest) (forkChanges, error) {
	var c forkChanges
	if q.Cwd != "" {
		abs, err := filepath.Abs(q.Cwd)
		if err != nil {
			return c, err
		}
		if st, er := os.Stat(abs); er != nil || !st.IsDir() {
			return c, dapi(api.CodeBadRequest, "cwd must be an existing directory")
		}
		c.cwd = abs
	}
	if q.Title != "" {
		title, err := agent.NormalizeTitle(q.Title)
		if err != nil {
			return c, dapi(api.CodeBadRequest, err.Error())
		}
		c.title = title
	}
	if q.Model != "" || q.Effort != "" {
		src.mu.Lock()
		cur := modelChoice{src.session.Provider, src.session.Model, src.session.Effort}
		src.mu.Unlock()
		pick, err := e.pickModel(q.Model, q.Effort, cur)
		if err != nil {
			return c, err
		}
		c.model = &pick
	}
	if q.System != "" {
		name, text, err := e.systemPrompt(q.System)
		if err != nil {
			return c, err
		}
		c.system = &[2]string{name, text}
	}
	c.noAsk = q.NoAsk
	return c, nil
}

// applyForkOverrides stores the changes on the new session before anything runs.
func (e *Engine) applyForkOverrides(s *runState, c forkChanges) error {
	s.commit.Lock()
	defer s.commit.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.model != nil {
		s.session.Provider, s.session.Model, s.session.Effort = c.model.provider, c.model.model, c.model.effort
	}
	if c.title != "" {
		s.session.Title = c.title
	}
	if c.system != nil {
		s.session.SystemName, s.session.SystemPrompt = c.system[0], c.system[1]
	}
	if c.noAsk {
		s.session.NoAsk = true
	}
	return e.save(s)
}
