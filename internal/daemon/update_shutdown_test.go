package daemon

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"meno/internal/api"
)

type updateBusyBackend struct {
	NotImplementedBackend
	busy             atomic.Bool
	entered, release chan struct{}
}

func (b *updateBusyBackend) HasWork(context.Context) bool { return b.busy.Load() }
func (b *updateBusyBackend) Message(context.Context, string, api.MessageRequest) (api.MessageResponse, error) {
	close(b.entered)
	<-b.release
	b.busy.Store(true)
	return api.MessageResponse{}, nil
}

func TestIdleShutdownSerializesWithIncomingWork(t *testing.T) {
	backend := &updateBusyBackend{entered: make(chan struct{}), release: make(chan struct{})}
	_, c := startTestServer(t, t.TempDir(), Options{Backend: backend})
	mutation := make(chan *http.Response, 1)
	go func() {
		r, _ := c.Post("http://unix/v1/sessions/s/messages", "application/json", strings.NewReader(`{"text":"hi","client_id":"c"}`))
		mutation <- r
	}()
	<-backend.entered
	shutdown := make(chan *http.Response, 1)
	go func() {
		r, _ := c.Post("http://unix/v1/shutdown/idle", "application/json", strings.NewReader(`{}`))
		shutdown <- r
	}()
	select {
	case r := <-shutdown:
		t.Fatalf("shutdown crossed mutation boundary: %v", r)
	case <-time.After(30 * time.Millisecond):
	}
	close(backend.release)
	if r := <-mutation; r == nil {
		t.Fatal("mutation failed")
	} else {
		r.Body.Close()
	}
	r := <-shutdown
	if r == nil {
		t.Fatal("shutdown request failed")
	}
	defer r.Body.Close()
	if r.StatusCode != 409 {
		t.Fatalf("status %d", r.StatusCode)
	}
}

func TestIdleShutdownRejectsLateMutations(t *testing.T) {
	stopped, release := make(chan struct{}), make(chan struct{})
	s, c := startTestServer(t, t.TempDir(), Options{OnStop: func() { close(stopped); <-release }})
	defer close(release)
	r, err := c.Post("http://unix/v1/shutdown/idle", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	<-stopped
	r, err = c.Post("http://unix/v1/sessions", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != 503 {
		t.Fatalf("late mutation status %d", r.StatusCode)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if f, err := AcquireOwnership(ctx, s.home, "daemon.lock"); err == nil {
		f.Close()
		t.Fatal("cleanup not complete but ownership released")
	}
}
