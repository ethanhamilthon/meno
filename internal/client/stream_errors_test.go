package client

import (
	"context"
	"errors"
	"fmt"
	"meno/internal/proto"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func sseServer(t *testing.T, hits *int32, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return &Client{HTTP: &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}}}}
}

// A callback error is the caller's decision: Stream returns it and does not reconnect.
func TestStreamReturnsCallbackErrorWithoutReconnect(t *testing.T) {
	var hits int32
	c := sseServer(t, &hits, "id: 1\nevent: text_delta\ndata: {}\n\n")
	sentinel := errors.New("consumer failed")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := c.Stream(ctx, "s1", 0, "", func(proto.Event) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("reconnected: %d requests", n)
	}
}

// A server that goes silent mid-stream must not hang the client forever: the
// idle watchdog has to close the body so the stream reconnects.
func TestSilentStreamIsClosedByTheIdleWatchdog(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "id: 1\nevent: text_delta\ndata: {}\n\n")
		w.(http.Flusher).Flush()
		if n == 1 {
			<-r.Context().Done() // silent: no data, no keepalive
		}
	}))
	t.Cleanup(srv.Close)
	c := &Client{IdleTimeout: 150 * time.Millisecond, HTTP: &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}}}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	seen := 0
	err := c.Stream(ctx, "s1", 0, "", func(proto.Event) error {
		seen++
		if atomic.LoadInt32(&hits) >= 2 {
			return errStopTest
		}
		return nil
	})
	if !errors.Is(err, errStopTest) {
		t.Fatalf("stream did not recover from a silent body: %v (hits=%d)", err, atomic.LoadInt32(&hits))
	}
}

var errStopTest = errors.New("stop test")
