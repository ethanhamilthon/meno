package engine

import (
	"meno/internal/api"
	"meno/internal/daemon"
	"time"
)

func dapi(code, msg string) error { return &daemon.APIError{Code: code, Message: msg} }
func item(s *runState) api.SessionItem {
	sess := s.session
	status := ""
	if s.running {
		status = s.phase
	}
	turns := s.session.Len()
	if s.running || s.compacting {
		turns = len(s.visibleTurns)
	}
	return api.SessionItem{
		ID:        sess.ID,
		Name:      sess.DisplayTitle(),
		Cwd:       s.cwd,
		Model:     sess.Model,
		Provider:  sess.Provider,
		Effort:    sess.Effort,
		Status:    status,
		Running:   s.running,
		NeedsYou:  s.ask != nil,
		TurnCount: turns,
		UpdatedAt: sess.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
