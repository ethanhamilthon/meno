package engine

import (
	"context"
	"meno/internal/daemon"
	"meno/internal/proto"
	"testing"
)

func TestDirectoryChangesArePublished(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	g := daemon.NewGlobalHub("boot", 100)
	e.global = g
	id := usedSession(t, e)
	if err := e.DeleteSession(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	ch, stop, _ := g.Subscribe(0, "boot")
	defer stop()
	var created, deleted bool
	for len(ch) > 0 {
		switch (<-ch).Type {
		case proto.EventSessionCreated:
			created = true
		case proto.EventSessionDeleted:
			deleted = true
		}
	}
	if !created || !deleted {
		t.Fatalf("created=%v deleted=%v", created, deleted)
	}
}
