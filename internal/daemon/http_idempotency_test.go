package daemon

import (
	"context"
	"meno/internal/api"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type countingBackend struct {
	NotImplementedBackend
	mu    sync.Mutex
	n     int
	cache *Idempotency
}

func (b *countingBackend) ApplyClientID(_ context.Context, key string, fn func() (any, error)) (any, error) {
	return b.cache.Do(key, fn)
}
func (b *countingBackend) CreateSession(_ context.Context, r api.SessionCreateRequest) (api.SessionCreateResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.n++
	return api.SessionCreateResponse{Session: api.SessionItem{ID: "only"}}, nil
}
func TestClientIDHTTPRetrySafety(t *testing.T) {
	b := &countingBackend{cache: NewIdempotency()}
	s, e := New(Options{Home: t.TempDir(), Backend: b})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/sessions", strings.NewReader(`{"cwd":"/tmp","client_id":"same"}`))
		s.routes().ServeHTTP(w, r)
		if w.Code != 202 {
			t.Fatalf("status %d", w.Code)
		}
	}
	b.mu.Lock()
	n := b.n
	b.mu.Unlock()
	if n != 1 {
		t.Fatalf("create executed %d times", n)
	}
}
func TestWorkRequestRequiresClientID(t *testing.T) {
	s, e := New(Options{Home: t.TempDir(), Backend: NewFakeBackend(NewHub("b", 8))})
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/sessions", strings.NewReader(`{"cwd":"/tmp"}`))
	s.routes().ServeHTTP(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), api.CodeBadRequest) {
		t.Fatalf("response %d %s", w.Code, w.Body.String())
	}
}
