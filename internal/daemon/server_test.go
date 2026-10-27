package daemon

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"meno/internal/api"
)

// startTestServer serves a Server in memory and returns an HTTP client for it.
func startTestServer(t *testing.T, _ string, opts Options) (*Server, *http.Client) {
	t.Helper()
	s := New(opts)
	dial, err := s.ServeMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Stop("test") })
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) { return dial(ctx) }}
	return s, &http.Client{Transport: tr}
}

func TestHelloAnswersOverMemory(t *testing.T) {
	hub := NewHub("", 20)
	_, c := startTestServer(t, "", Options{Backend: NewFakeBackend(hub), Hub: hub, Version: "test"})
	resp, err := c.Get("http://unix/v1/hello")
	if err != nil {
		t.Fatal(err)
	}
	var h api.Hello
	_ = json.NewDecoder(resp.Body).Decode(&h)
	resp.Body.Close()
	if h.Protocol != api.Protocol || h.Version != "test" {
		t.Fatalf("hello %+v", h)
	}
}

func TestStopClosesTheListener(t *testing.T) {
	s := New(Options{})
	dial, err := s.ServeMem()
	if err != nil {
		t.Fatal(err)
	}
	s.Stop("test")
	if _, err := dial(context.Background()); err == nil {
		t.Fatal("a stopped server must refuse connections")
	}
}
