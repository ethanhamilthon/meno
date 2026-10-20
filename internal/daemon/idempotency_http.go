package daemon

import (
	"meno/internal/proto"
	"net/http"
)

func (s *Server) idempotent(w http.ResponseWriter, r *http.Request, key string, fn func() (any, error), status int) {
	if key == "" {
		s.result(w, nil, &APIError{Code: proto.CodeBadRequest, Message: "client_id is required"})
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
func sessionCreateKey(q proto.SessionCreateRequest) string { return q.ClientID }
func messageKey(q proto.MessageRequest) string             { return q.ClientID }
func forkKey(q proto.ForkRequest) string                   { return q.ClientID }
func compactKey(q proto.CompactRequest) string             { return q.ClientID }
