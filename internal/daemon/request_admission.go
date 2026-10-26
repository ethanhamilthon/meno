package daemon

import (
	"net/http"
	"strings"

	"meno/internal/api"
)

// Finite requests are counted, not locked for their lifetime: a long ask/wait
// request must never block the answer or cancellation that lets it finish.
func (s *Server) mutationGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/shutdown/idle" || strings.HasSuffix(r.URL.Path, "/events") {
			next.ServeHTTP(w, r)
			return
		}
		s.admission.Lock()
		if s.stopping {
			s.admission.Unlock()
			s.result(w, nil, &APIError{Code: api.CodeDaemonStopping, Message: "daemon is stopping; reopen meno"})
			return
		}
		s.requests++
		s.touchIdle()
		s.admission.Unlock()
		defer func() {
			s.admission.Lock()
			s.requests--
			s.touchIdle()
			close(s.requestChanged)
			s.requestChanged = make(chan struct{})
			s.admission.Unlock()
		}()
		next.ServeHTTP(w, r)
	})
}
