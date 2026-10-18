package app

import (
	"context"
	"testing"
	"time"

	"meno/internal/config"
	"meno/internal/mcp"
)

// A server that is slow to start holds up the message that builds the prompt,
// not every message after it.
func TestOrdinaryTurnDoesNotWaitForStartingServers(t *testing.T) {
	m := mcp.NewManager()
	m.BeginStart("slow")
	defer m.EndStart("slow")
	src := worldSrc{Paths: config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}, MCP: m, Now: time.Now()}
	idle := src
	idle.MCP = mcp.NewManager()
	first := prepare(context.Background(), promptState{}, idle, false) // a reset point, nothing starting
	t0 := time.Now()
	prepare(context.Background(), first.State, src, true)
	if d := time.Since(t0); d > 500*time.Millisecond {
		t.Fatalf("an ordinary turn waited %v for a starting server", d)
	}
}
