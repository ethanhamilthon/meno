package daemon

import (
	"encoding/json"
	"meno/internal/proto"
	"net/http"
)

// registerExtraRoutes adds opaque agent, config, search, and debug endpoints.
func (s *Server) registerExtraRoutes(m *http.ServeMux) {
	m.Handle("/v1/sessions/{id}/{resource...}", s.resourceHandler())
	for _, base := range []string{"session-state", "settings", "providers", "login", "skills", "mcp/servers", "models"} {
		m.Handle("/v1/"+base, s.resourceHandler())
		m.Handle("/v1/"+base+"/{resource...}", s.resourceHandler())
	}
	m.HandleFunc("GET /v1/fs/search", s.fileSearch)
	m.HandleFunc("GET /v1/debug/requests", s.debugRequests)
}
func (s *Server) resourceHandler() http.Handler { return http.HandlerFunc(s.resource) }
func (s *Server) fileSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	data, _ := json.Marshal(proto.FileSearchRequest{Root: q.Get("root"), Query: q.Get("q")})
	v, e := s.backend.Resource(r.Context(), r.Method, "/v1/fs/search", r.URL.RawQuery, proto.ResourceRequest{Data: data, Token: bearer(r)})
	s.result(w, v.Data, e)
}
func (s *Server) debugRequests(w http.ResponseWriter, r *http.Request) {
	v, e := s.backend.Resource(r.Context(), r.Method, r.URL.Path, r.URL.RawQuery, proto.ResourceRequest{Token: bearer(r)})
	s.result(w, v.Data, e)
}
