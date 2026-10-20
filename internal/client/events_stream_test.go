package client

import (
	"context"
	"encoding/json"
	"meno/internal/daemon"
	"meno/internal/proto"
	"testing"
	"time"
)

func TestAttachSnapshotThenNoGapStream(t *testing.T) {
	home := t.TempDir()
	hub := daemon.NewHub("boot", 2000)
	backend := daemon.NewFakeBackend(hub)
	srv, c := startClientTestServer(t, home, hub, backend)
	created, e := c.CreateSession(context.Background(), proto.SessionCreateRequest{Cwd: "/tmp", ClientID: "create"})
	if e != nil {
		t.Fatal(e)
	}
	got := make(chan proto.Event, 8)
	runCtx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- c.Attach(runCtx, created.Session.ID, &Mirror{}, func(ev proto.Event) error {
			got <- ev
			if ev.Type == proto.EventDone {
				stop()
			}
			return nil
		})
	}()
	time.Sleep(20 * time.Millisecond)
	if _, e = c.Message(context.Background(), created.Session.ID, proto.MessageRequest{Text: "hello", ClientID: "message"}); e != nil {
		t.Fatal(e)
	}
	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("attach did not receive done")
	}
	seen := map[string]int{}
	for {
		select {
		case ev := <-got:
			seen[ev.Type]++
		default:
			goto collected
		}
	}
collected:
	if seen[proto.EventTextDelta] != 1 || seen[proto.EventTurnAdded] != 1 || seen[proto.EventDone] != 1 {
		b, _ := json.Marshal(seen)
		t.Fatalf("missing streamed events: %s", b)
	}
	srv.Stop("test")
}
