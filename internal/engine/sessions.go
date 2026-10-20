package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/modes"
	"meno/internal/proto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (e *Engine) ListSessions(ctx context.Context, q proto.SessionsListRequest) (proto.SessionsListResponse, error) {
	rows, err := agent.ListSessions(e.home)
	if err != nil {
		return proto.SessionsListResponse{}, err
	}
	out := proto.SessionsListResponse{Sessions: []proto.SessionItem{}}
	for _, r := range rows {
		e.mu.Lock()
		s := e.sessions[r.ID]
		e.mu.Unlock()
		var it proto.SessionItem
		if s != nil {
			s.mu.Lock()
			it = item(s)
			s.mu.Unlock()
		} else {
			it = proto.SessionItem{ID: r.ID, Name: r.Title, Cwd: r.Cwd, Model: r.Model, UpdatedAt: r.UpdatedAt.UTC().Format(time.RFC3339Nano)}
		}
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
func (e *Engine) CreateSession(ctx context.Context, q proto.SessionCreateRequest) (proto.SessionCreateResponse, error) {
	cwd := q.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return proto.SessionCreateResponse{}, err
	}
	if st, er := os.Stat(abs); er != nil || !st.IsDir() {
		return proto.SessionCreateResponse{}, api(proto.CodeBadRequest, "cwd must be an existing directory")
	}
	model := q.Model
	if model == "" {
		model = e.settings.Model
	}
	sess := agent.NewSession(abs, model)
	sess.Provider = e.settings.Provider
	sess.Effort = e.settings.Effort
	sess.Modes, err = validateModes(q.Modes)
	if err != nil {
		return proto.SessionCreateResponse{}, err
	}
	if err = agent.SaveSession(e.home, sess); err != nil {
		return proto.SessionCreateResponse{}, err
	}
	s, err := e.load(ctx, sess.ID)
	if err != nil {
		return proto.SessionCreateResponse{}, err
	}
	s.mu.Lock()
	s.env = cloneMap(q.Env)
	s.token.groups = groupsFor(sess.Modes)
	result := proto.SessionCreateResponse{Session: item(s)}
	s.mu.Unlock()
	return result, nil
}
func validateModes(names []string) ([]string, error) {
	known := map[string]bool{}
	for _, m := range modes.Catalog() {
		known[m.Name] = true
	}
	out := []string{}
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimPrefix(n, "#")
		if n == modes.Control {
			out = nil
			continue
		}
		if !known[n] {
			return nil, api(proto.CodeBadRequest, "unknown mode #"+n)
		}
		if !seen[n] {
			out = append(out, n)
			seen[n] = true
		}
	}
	return out, nil
}
