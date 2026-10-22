package engine

import (
	"context"
	"meno/internal/api"
	"meno/internal/daemon"
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
		case api.EventSessionCreated:
			created = true
		case api.EventSessionDeleted:
			deleted = true
		}
	}
	if !created || !deleted {
		t.Fatalf("created=%v deleted=%v", created, deleted)
	}
}
