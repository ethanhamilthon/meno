package client

import (
	"context"
	"meno/internal/daemon"
	"meno/internal/proto"
	"testing"
	"time"
)

func TestAttachReloadsAfterRingResync(t *testing.T) {
	home := t.TempDir()
	hub := daemon.NewHub("boot", 2)
	backend := daemon.NewFakeBackend(hub)
	srv, c := startClientTestServer(t, home, hub, backend)
	created, e := c.CreateSession(context.Background(), proto.SessionCreateRequest{Cwd: "/tmp", ClientID: "create"})
	if e != nil {
		t.Fatal(e)
	}
	m := &Mirror{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Attach(ctx, created.Session.ID, m, nil) }()
	time.Sleep(25 * time.Millisecond)
	for i := 0; i < 8; i++ {
		hub.Publish(created.Session.ID, proto.EventTextDelta, proto.TextDeltaData{Text: "x"})
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("attach did not stop")
	}
	if m.Snapshot().NeedsResync {
		t.Fatal("mirror was not reloaded after resync")
	}
	srv.Stop("test")
}
