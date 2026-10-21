package daemon

import (
	"context"
	"net/http"

	"meno/internal/proto"
)

// mutationGate makes admission and the idle check one coherent boundary. Streams
// and read-only requests do not hold the gate while waiting for events.
func (s *Server) mutationGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" || r.URL.Path == "/v1/shutdown/idle" {
			next.ServeHTTP(w, r)
			return
		}
		s.admission.RLock()
		defer s.admission.RUnlock()
		if s.stopping {
			s.result(w, nil, &APIError{Code: proto.CodeDaemonStopping, Message: "daemon is stopping; reopen meno after updating"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) shutdownIdle(w http.ResponseWriter, r *http.Request) {
	s.admission.Lock()
	defer s.admission.Unlock()
	if r.Context().Err() != nil {
		return
	}
	busy := s.backend.HasWork(r.Context())
	if b, ok := s.backend.(interface {
		HasUpdateWork(context.Context) (bool, error)
	}); ok {
		var err error
		busy, err = b.HasUpdateWork(r.Context())
		if err != nil {
			s.result(w, nil, err)
			return
		}
	}
	if busy {
		s.result(w, nil, &APIError{Code: proto.CodeConflict, Message: "daemon has active runs or background tasks; finish or stop them and retry the update"})
		return
	}
	s.stopping = true
	writeJSON(w, 200, proto.Ack{OK: true})
	go s.Stop("update")
}
