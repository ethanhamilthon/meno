package client

import (
	"context"
	"meno/internal/daemon"
	"net"
	"net/http"
	"testing"
	"time"
)

func startClientTestServer(t *testing.T, home string, hub *daemon.Hub, backend daemon.Backend) (*daemon.Server, *Client) {
	t.Helper()
	srv, e := daemon.New(daemon.Options{Home: home, Hub: hub, Backend: backend, Version: "test"})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { srv.Stop("test") })
	go func() { _ = srv.Serve(ctx) }()
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", srv.Socket())
	}}
	c := New(home)
	c.HTTP = &http.Client{Transport: tr}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, e = c.Hello(context.Background()); e == nil {
			return srv, c
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("server did not start")
	return nil, nil
}
