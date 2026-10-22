package daemon

import (
	"meno/internal/api"
	"net/http"
)

func (s *Server) idempotent(w http.ResponseWriter, r *http.Request, key string, fn func() (any, error), status int) {
	if key == "" {
		s.result(w, nil, &APIError{Code: api.CodeBadRequest, Message: "client_id is required"})
		return
	}
	v, e := s.backend.ApplyClientID(r.Context(), key, fn)
	if e != nil {
		s.result(w, nil, e)
		return
	}
	if status == 202 {
		writeJSON(w, status, v)
		return
	}
	writeJSON(w, 200, v)
}
func sessionCreateKey(q api.SessionCreateRequest) string { return q.ClientID }
func messageKey(q api.MessageRequest) string             { return q.ClientID }
func forkKey(q api.ForkRequest) string                   { return q.ClientID }
func compactKey(q api.CompactRequest) string             { return q.ClientID }
