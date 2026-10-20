package engine

import (
	"meno/internal/daemon"
	"meno/internal/proto"
	"time"
)

func api(code, msg string) error { return &daemon.APIError{Code: code, Message: msg} }
func item(s *runState) proto.SessionItem {
	sess := s.session
	status := ""
	if s.running {
		status = s.phase
	}
	return proto.SessionItem{ID: sess.ID, Name: sess.DisplayTitle(), Cwd: s.cwd, Model: sess.Model, Status: status, Running: s.running, NeedsYou: s.ask != nil, UpdatedAt: sess.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}
