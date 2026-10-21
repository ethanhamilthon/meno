package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnsureProtocolMismatchMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"protocol": 99})
	}))
	defer srv.Close()
	c := New(t.TempDir())
	c.HTTP = srv.Client()
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(context.Background())
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(srv.URL, "http://")
		return http.DefaultTransport.RoundTrip(clone)
	})
	err := c.check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "meno daemon restart") {
		t.Fatalf("unexpected mismatch error %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Whichever side is older, the client refuses to talk and says what to do.
func TestEnsureRefusesOlderDaemon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"protocol": 1})
	}))
	defer srv.Close()
	c := New(t.TempDir())
	c.HTTP = srv.Client()
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(context.Background())
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(srv.URL, "http://")
		return http.DefaultTransport.RoundTrip(clone)
	})
	if err := c.check(context.Background()); err == nil || !strings.Contains(err.Error(), "daemon restart") {
		t.Fatalf("an older daemon was accepted: %v", err)
	}
}
