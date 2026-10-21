package engine

import (
	"context"
	"meno/internal/agent"
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
func (e *Engine) CreateSession(ctx context.Context, q proto.SessionCreateRequest) (proto.SessionCreateResponse, error) {
	if err := e.reloadConfig(); err != nil {
		return proto.SessionCreateResponse{}, api(proto.CodeInternal, err.Error())
	}
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
		model = e.cfg().Model
	}
	sess := agent.NewSession(abs, model)
	sess.Provider = e.cfg().Provider
	sess.Effort = e.cfg().Effort
	s, err := e.adopt(sess)
	if err != nil {
		return proto.SessionCreateResponse{}, err
	}
	s.mu.Lock()
	s.env = cloneMap(q.Env)
	s.token.groups = groupsFor(nil)
	result := proto.SessionCreateResponse{Session: item(s)}
	s.mu.Unlock()
	return result, nil
}
