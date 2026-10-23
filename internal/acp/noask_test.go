package acp

import (
	"context"
	"sync"
	"testing"

	"meno/internal/api"
)

type recordingBackend struct {
	scriptBackend
	mu2  sync.Mutex
	made []api.SessionCreateRequest
}

func (b *recordingBackend) CreateSession(ctx context.Context, q api.SessionCreateRequest) (api.SessionCreateResponse, error) {
	b.mu2.Lock()
	b.made = append(b.made, q)
	b.mu2.Unlock()
	return b.scriptBackend.CreateSession(ctx, q)
}

// session/new asks the daemon for a session without the ask tool exactly when
// the adapter runs with --no-ask.
func TestSessionNewPassesNoAsk(t *testing.T) {
	for _, noAsk := range []bool{false, true} {
		b := &recordingBackend{scriptBackend: scriptBackend{up: make(chan struct{})}}
		p := startPeerWith(t, b, Options{Version: "test", NoAsk: noAsk})
		newSessionOn(t, p)
		b.mu2.Lock()
		n, got := len(b.made), false
		if n == 1 {
			got = b.made[0].NoAsk
		}
		b.mu2.Unlock()
		if n != 1 || got != noAsk {
			t.Errorf("noAsk=%v: %d requests, NoAsk=%v", noAsk, n, got)
		}
	}
}
