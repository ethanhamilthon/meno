package tui

import (
	"context"
	"meno/internal/client"
	"testing"
)

func TestOnlyCountedCompletionMarksUnseen(t *testing.T) {
	for _, counted := range []bool{false, true} {
		a, rt := queueTestApp()
		a.projectionEffects(rt, client.ProjectionEffects{Finished: true, Counted: counted})
		if rt.Unseen != counted {
			t.Fatalf("counted=%v unseen=%v", counted, rt.Unseen)
		}
	}
}

func TestComposerSubmitCountsCompletion(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	typeRunes(a, "human submission")
	a.submit()
	pump(t, a, func() bool { return runDone(a) })
	var completed int64
	err := a.rpc(func(ctx context.Context, c *client.Client) error {
		snap, err := c.Snapshot(ctx, a.st.Runtime.ID())
		completed = snap.Session.Completed
		return err
	})
	if err != nil || completed != 1 {
		t.Fatalf("TUI submit completion=%d: %v", completed, err)
	}
}
