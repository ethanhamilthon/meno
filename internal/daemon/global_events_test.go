package daemon

import (
	"meno/internal/proto"
	"testing"
)

func TestGlobalHubResumeAndResync(t *testing.T) {
	g := NewGlobalHub("boot", 2)
	g.Publish("s", proto.EventStatus, map[string]bool{"running": true})
	ch, cancel, resync := g.Subscribe(1, "boot")
	defer cancel()
	if resync {
		t.Fatal("unexpected resync")
	}
	g.Publish("s", proto.EventStatus, map[string]bool{"running": false})
	e := <-ch
	if e.Seq != 2 {
		t.Fatalf("seq %d", e.Seq)
	}
	g.Publish("s", proto.EventDone, map[string]bool{})
	g.Publish("s", proto.EventDone, map[string]bool{})
	_, cancel, resync = g.Subscribe(0, "boot")
	cancel()
	if !resync {
		t.Fatal("expected overflow resync")
	}
}
