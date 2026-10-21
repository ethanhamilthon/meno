package daemon

import (
	"encoding/json"
	"io"
	"meno/internal/proto"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /v1/hello", s.hello)
	m.HandleFunc("POST /v1/shutdown", s.shutdown)
	m.HandleFunc("POST /v1/shutdown/idle", s.shutdownIdle)
	m.HandleFunc("GET /v1/sessions", s.list)
	m.HandleFunc("POST /v1/sessions", s.create)
	m.HandleFunc("GET /v1/sessions/{id}", s.snapshot)
	m.HandleFunc("PATCH /v1/sessions/{id}", s.patch)
	m.HandleFunc("DELETE /v1/sessions/{id}", s.delete)
	m.HandleFunc("POST /v1/sessions/{id}/fork", s.fork)
	m.HandleFunc("GET /v1/sessions/{id}/turns", s.turns)
	m.HandleFunc("POST /v1/sessions/{id}/messages", s.message)
	m.HandleFunc("POST /v1/sessions/{id}/stop", s.stopSession)
	m.HandleFunc("POST /v1/sessions/{id}/compact", s.compact)
	m.HandleFunc("POST /v1/sessions/{id}/asks/{aid}", s.answer)
	m.HandleFunc("GET /v1/sessions/{id}/events", s.sessionEvents)
	m.HandleFunc("GET /v1/events", s.globalEvents)
	s.registerExtraRoutes(m)
	m.Handle("/v1/", s.resourceHandler())
	return s.mutationGate(m)
}
func (s *Server) hello(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, proto.Hello{Protocol: proto.Protocol, Version: s.version, BootID: s.bootID, PID: s.pid, StartedAt: s.started.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")})
}
func (s *Server) shutdown(w http.ResponseWriter, r *http.Request) {
	var q proto.ShutdownRequest
	if r.Body != nil {
		if e := decode(r, &q); e != nil {
			s.result(w, nil, &APIError{Code: proto.CodeBadRequest, Message: e.Error()})
			return
		}
	}
	writeJSON(w, 200, proto.Ack{OK: true})
	go s.Stop(q.Reason)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	v, e := s.backend.ListSessions(r.Context(), proto.SessionsListRequest{Cwd: q.Get("cwd"), Query: q.Get("q"), Limit: limit})
	s.result(w, v, e)
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var q proto.SessionCreateRequest
	if !body(w, r, &q) {
		return
	}
	s.idempotent(w, r, sessionCreateKey(q), func() (any, error) { return s.backend.CreateSession(r.Context(), q) }, 202)
}
func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	v, e := s.backend.Snapshot(r.Context(), r.PathValue("id"))
	s.result(w, v, e)
}
func (s *Server) patch(w http.ResponseWriter, r *http.Request) {
	var q proto.PatchSessionRequest
	if !body(w, r, &q) {
		return
	}
	v, e := s.backend.PatchSession(r.Context(), r.PathValue("id"), q)
	s.result(w, v, e)
}
func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	s.result(w, proto.Ack{OK: true}, s.backend.DeleteSession(r.Context(), r.PathValue("id")))
}
func (s *Server) fork(w http.ResponseWriter, r *http.Request) {
	var q proto.ForkRequest
	if !body(w, r, &q) {
		return
	}
	s.idempotent(w, r, forkKey(q), func() (any, error) { return s.backend.Fork(r.Context(), r.PathValue("id"), q) }, 202)
}
func (s *Server) turns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, _ := strconv.Atoi(q.Get("from"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	v, e := s.backend.Turns(r.Context(), r.PathValue("id"), proto.TurnsPageRequest{From: from, Limit: limit})
	s.result(w, v, e)
}
func (s *Server) message(w http.ResponseWriter, r *http.Request) {
	var q proto.MessageRequest
	if !body(w, r, &q) {
		return
	}
	s.idempotent(w, r, messageKey(q), func() (any, error) { return s.backend.Message(r.Context(), r.PathValue("id"), q) }, 202)
}
func (s *Server) stopSession(w http.ResponseWriter, r *http.Request) {
	var q proto.StopRequest
	if r.Body != nil {
		if e := decode(r, &q); e != nil {
			s.result(w, nil, &APIError{Code: proto.CodeBadRequest, Message: e.Error()})
			return
		}
	}
	s.result(w, proto.Ack{OK: true}, s.backend.Stop(r.Context(), r.PathValue("id"), q))
}
func (s *Server) compact(w http.ResponseWriter, r *http.Request) {
	var q proto.CompactRequest
	if !body(w, r, &q) {
		return
	}
	s.idempotent(w, r, compactKey(q), func() (any, error) { return s.backend.Compact(r.Context(), r.PathValue("id"), q) }, 200)
}
func (s *Server) answer(w http.ResponseWriter, r *http.Request) {
	var q proto.AskAnswerRequest
	if !body(w, r, &q) {
		return
	}
	s.result(w, proto.Ack{OK: true}, s.backend.AnswerAsk(r.Context(), r.PathValue("id"), r.PathValue("aid"), q))
}
func (s *Server) resource(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" && r.Method != "PUT" && r.Method != "PATCH" && r.Method != "DELETE" {
		http.NotFound(w, r)
		return
	}
	var q proto.ResourceRequest
	if r.Body != nil && r.Method != "GET" {
		if !body(w, r, &q) {
			return
		}
	}
	q.Token = bearer(r)
	q.Call = r.Header.Get("X-Meno-Call")
	q.SessionID = r.PathValue("id")
	v, e := s.backend.Resource(r.Context(), r.Method, r.URL.Path, r.URL.RawQuery, q)
	s.result(w, v, e)
}
func (s *Server) result(w http.ResponseWriter, v any, e error, status ...int) {
	if e != nil {
		a := apiError(e)
		writeJSON(w, a.Status, proto.ErrorEnvelope{Error: proto.ErrorBody{Code: a.Code, Message: a.Message}})
		return
	}
	code := 200
	if len(status) > 0 {
		code = status[0]
	}
	writeJSON(w, code, v)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	d := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func body(w http.ResponseWriter, r *http.Request, v any) bool {
	if e := decode(r, v); e != nil {
		writeJSON(w, 400, proto.ErrorEnvelope{Error: proto.ErrorBody{Code: proto.CodeBadRequest, Message: e.Error()}})
		return false
	}
	return true
}

func bearer(r *http.Request) string {
	if h := r.Header.Get("X-Meno-Token"); h != "" {
		return h
	}
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) >= len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
