package daemon

import (
	"context"
	"meno/internal/api"
	"net/http/httptest"
	"strings"
	"testing"
)

type messageClientBackend struct {
	countingBackend
	clients []string
}

func (b *messageClientBackend) Message(_ context.Context, _ string, q api.MessageRequest) (api.MessageResponse, error) {
	b.clients = append(b.clients, q.Client)
	return api.MessageResponse{}, nil
}
func TestAgentTokenOverridesMessageClient(t *testing.T) {
	b := &messageClientBackend{countingBackend: countingBackend{cache: NewIdempotency()}}
	s, err := New(Options{Home: t.TempDir(), Backend: b})
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", "agent-token"} {
		r := httptest.NewRequest("POST", "/v1/sessions/s/messages", strings.NewReader(`{"text":"hello","client":"tui","client_id":"`+token+`message"}`))
		if token != "" {
			r.Header.Set("X-Meno-Token", token)
		}
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, r)
		if w.Code != 202 {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	if len(b.clients) != 2 || b.clients[0] != "tui" || b.clients[1] != "agent" {
		t.Fatalf("clients %v", b.clients)
	}
}
