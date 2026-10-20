package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientProtocolMismatchRequiresRestart(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"protocol":99}`)) }))
	defer s.Close()
	c := New(t.TempDir())
	c.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		r := req.Clone(context.Background())
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(s.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})}
	if e := c.check(context.Background()); e == nil || !strings.Contains(e.Error(), "meno daemon restart") {
		t.Fatalf("unexpected error %v", e)
	}
}
