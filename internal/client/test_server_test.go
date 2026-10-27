package client

import (
	"context"
	"testing"

	"meno/internal/daemon"
)

func startClientTestServer(t *testing.T, _ string, hub *daemon.Hub, backend daemon.Backend) (*daemon.Server, *Client) {
	t.Helper()
	srv := daemon.New(daemon.Options{Hub: hub, Backend: backend, Version: "test"})
	dial, err := srv.ServeMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Stop("test") })
	c := New(dial)
	if _, err := c.Hello(context.Background()); err != nil {
		t.Fatal(err)
	}
	return srv, c
}
