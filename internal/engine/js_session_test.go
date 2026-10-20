package engine

import (
	"context"
	"testing"
)

// jsSession turns a session into one that still works through the js tool.
func jsSession(t *testing.T, e *Engine, id string) {
	t.Helper()
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.session.Tool = "js"
	s.mu.Unlock()
}
