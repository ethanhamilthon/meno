package acp

import (
	"context"
	"sync"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/client"
)

// slowBackend answers Message only when released, like a daemon that has
// already accepted a run while the adapter's connection closes.
type slowBackend struct {
	Backend
	release chan struct{}
	entered chan struct{}
	mu      sync.Mutex
	stopped []string
}

func (b *slowBackend) CreateSession(context.Context, api.SessionCreateRequest) (api.SessionCreateResponse, error) {
	return api.SessionCreateResponse{Session: api.SessionItem{ID: "s1"}}, nil
}
func (b *slowBackend) Models(context.Context, api.ModelsRequest) (api.ModelsResponse, error) {
	return api.ModelsResponse{}, nil
}
func (b *slowBackend) Skills(context.Context) (api.SkillsResponse, error) {
	return api.SkillsResponse{}, nil
}
func (b *slowBackend) Follow(ctx context.Context, _ string, recv func(client.ProjectionUpdate) error) error {
	_ = recv(client.ProjectionUpdate{Reset: true, Changed: client.AllChanged})
	<-ctx.Done()
	return ctx.Err()
}
func (b *slowBackend) Message(ctx context.Context, _ string, _ api.MessageRequest) (api.MessageResponse, error) {
	close(b.entered)
	<-b.release
	if ctx.Err() != nil {
		return api.MessageResponse{}, ctx.Err() // the submission must not be cancelled
	}
	return api.MessageResponse{RequestID: "r42"}, nil
}
func (b *slowBackend) Stop(_ context.Context, _ string, q api.StopRequest) error {
	b.mu.Lock()
	b.stopped = append(b.stopped, q.RequestID)
	b.mu.Unlock()
	return nil
}

func TestDisconnectBeforeRequestIDStillStopsTheRun(t *testing.T) {
	b := &slowBackend{release: make(chan struct{}), entered: make(chan struct{})}
	p := startPeer(t, b)
	id := newSessionOn(t, p)
	p.start("session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text", "text": "go"}}})
	<-b.entered
	_ = p.in.Close()
	time.Sleep(50 * time.Millisecond)
	close(b.release)
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.stopped) != 1 || b.stopped[0] != "r42" {
		t.Fatalf("stopped = %v, want [r42]", b.stopped)
	}
}
