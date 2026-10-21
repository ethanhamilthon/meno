package engine

import (
	"context"
	"meno/internal/proto"
	"testing"
)

func TestSecondCompactIsRejectedWhileFirstRuns(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	s.compacting = true
	s.mu.Unlock()
	_, err := e.Compact(context.Background(), id, proto.CompactRequest{})
	if err == nil {
		t.Fatal("expected a conflict")
	}
}
