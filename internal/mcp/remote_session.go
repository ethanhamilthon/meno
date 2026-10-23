package mcp

import "context"

// recoverSession starts a new session after expired was rejected with 404.
// Concurrent callers share one restart: whoever finds the session already
// replaced just retries.
func (r *remote) recoverSession(ctx context.Context, expired string) error {
	r.reinitMu.Lock()
	defer r.reinitMu.Unlock()
	r.mu.Lock()
	current := r.session
	if current == expired {
		r.session = "" // the initialize request must carry no session ID
	}
	r.mu.Unlock()
	if current != expired {
		return nil
	}
	return r.reinit(ctx)
}
