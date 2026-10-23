package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/api"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (e *Engine) ListSessions(ctx context.Context, q api.SessionsListRequest) (api.SessionsListResponse, error) {
	rows, err := agent.ListSessions(e.home)
	if err != nil {
		return api.SessionsListResponse{}, err
	}
	out := api.SessionsListResponse{Sessions: []api.SessionItem{}}
	for _, r := range rows {
		e.mu.Lock()
		s := e.sessions[r.ID]
		e.mu.Unlock()
		var it api.SessionItem
		if s != nil {
			s.mu.Lock()
			it = item(s)
			s.mu.Unlock()
		} else {
			it = api.SessionItem{
				ID:        r.ID,
				Name:      r.Title,
				Cwd:       r.Cwd,
				Model:     r.Model,
				Provider:  r.Provider,
				Effort:    r.Effort,
				NoAsk:     r.NoAsk,
				SpawnedBy: r.SpawnedBy,
				ForkOf:    r.ForkSource,
				System:    r.SystemName,
				TurnCount: r.Turns,
				UpdatedAt: r.UpdatedAt.UTC().Format(time.RFC3339Nano),
			}
		}
		it.Completed, it.ReadThrough = e.attention(it.ID)
		if q.Cwd != "" && it.Cwd != q.Cwd {
			continue
		}
		if q.Query != "" && !strings.Contains(strings.ToLower(it.Name+" "+it.Cwd), strings.ToLower(q.Query)) {
			continue
		}
		out.Sessions = append(out.Sessions, it)
		if q.Limit > 0 && len(out.Sessions) >= q.Limit {
			break
		}
	}
	return out, nil
}
func (e *Engine) CreateSession(ctx context.Context, q api.SessionCreateRequest) (api.SessionCreateResponse, error) {
	if err := e.reloadConfig(); err != nil {
		return api.SessionCreateResponse{}, dapi(api.CodeInternal, err.Error())
	}
	cwd := q.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return api.SessionCreateResponse{}, err
	}
	if st, er := os.Stat(abs); er != nil || !st.IsDir() {
		return api.SessionCreateResponse{}, dapi(api.CodeBadRequest, "cwd must be an existing directory")
	}
	cfg := e.cfg()
	choice, err := e.pickModel(q.Model, q.Effort, modelChoice{cfg.Provider, cfg.Model, cfg.Effort})
	if err != nil {
		return api.SessionCreateResponse{}, err
	}
	title := ""
	if q.Title != "" {
		if title, err = agent.NormalizeTitle(q.Title); err != nil {
			return api.SessionCreateResponse{}, dapi(api.CodeBadRequest, err.Error())
		}
	}
	systemName, systemText, err := e.systemPrompt(q.System)
	if err != nil {
		return api.SessionCreateResponse{}, err
	}
	if err = e.checkSpawn(ctx, q.SpawnedBy); err != nil {
		return api.SessionCreateResponse{}, err
	}
	sess := agent.NewSession(abs, choice.model)
	sess.Provider = choice.provider
	sess.Effort = choice.effort
	sess.NoAsk = q.NoAsk
	sess.Title = title
	sess.SpawnedBy = q.SpawnedBy
	sess.SystemName, sess.SystemPrompt = systemName, systemText
	s, err := e.adopt(sess)
	if err != nil {
		return api.SessionCreateResponse{}, err
	}
	s.mu.Lock()
	s.env = cloneMap(q.Env)
	s.token.groups = groupsFor(nil)
	result := api.SessionCreateResponse{Session: item(s)}
	s.mu.Unlock()
	return result, nil
}
