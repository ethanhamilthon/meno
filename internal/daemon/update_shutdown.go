package daemon

import (
	"net/http"

	"meno/internal/api"
)

func (s *Server) shutdownIdle(w http.ResponseWriter, r *http.Request) {
	s.admission.Lock()
	defer s.admission.Unlock()
	if r.Context().Err() != nil {
		return
	}
	// Wait for work admission to settle without blocking requests that complete
	// an already active run (for example an answer to a long-lived ask request).
	for s.requests > 0 {
		busy, err := s.hasCompleteWork(r.Context())
		if err != nil {
			s.result(w, nil, err)
			return
		}
		if busy {
			s.result(w, nil, &APIError{Code: api.CodeConflict, Message: "daemon has active runs or background tasks; finish or stop them and retry the update"})
			return
		}
		changed := s.requestChanged
		s.admission.Unlock()
		select {
		case <-changed:
		case <-r.Context().Done():
		}
		s.admission.Lock()
		if r.Context().Err() != nil {
			return
		}
	}
	busy, err := s.hasCompleteWork(r.Context())
	if err != nil {
		s.result(w, nil, err)
		return
	}
	if busy {
		s.result(w, nil, &APIError{Code: api.CodeConflict, Message: "daemon has active runs or background tasks; finish or stop them and retry the update"})
		return
	}
	s.stopping = true
	writeJSON(w, 200, api.Ack{OK: true})
	go s.Stop("update")
}
