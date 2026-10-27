// Package client provides typed calls and resumable streams for the engine's
// protocol. The engine runs in the same process (a daemon.MemListener) or is
// the other end of a descriptor inherited by a bash call.
package client

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Client talks to one engine.
type Client struct {
	HTTP *http.Client
	// IdleTimeout closes an event stream that sends nothing (not even a
	// keepalive) for this long, so the stream reconnects. Zero means 45s.
	IdleTimeout time.Duration
}

// New creates a client that reaches the engine through dial.
func New(dial func(context.Context) (net.Conn, error)) *Client {
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) { return dial(ctx) }}
	return &Client{HTTP: &http.Client{Transport: tr}}
}
