package app

import (
	"context"
	"testing"
	"time"

	"meno/internal/config"
	"meno/internal/mcp"
)

func TestPolicySignatureIsSortedAndDistinguishesNil(t *testing.T) {
	if policySignature(nil) != "*" || policySignature([]string{}) != "" {
		t.Fatal("nil and empty policies must remain distinct")
	}
	if policySignature([]string{"goal", "agents"}) != policySignature([]string{"agents", "goal"}) {
		t.Fatal("policy signature must not depend on order")
	}
}

func TestPrepareRebuildsPromptWhenPolicyChanges(t *testing.T) {
	src := worldSrc{Paths: config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}, MCP: mcp.NewManager(), Now: time.Now()}
	first := prepare(context.Background(), promptState{}, src, false)
	changedSrc := src
	changedSrc.Only = []string{"agents", "goal"}
	changedSrc.PolicySig = policySignature(changedSrc.Only)
	changed := prepare(context.Background(), first.State, changedSrc, true)
	if changed.State.Frozen == first.State.Frozen {
		t.Fatal("changed policy must rebuild the prompt")
	}
	if changed.Event != "" {
		t.Fatalf("policy change must not emit world events: %s", changed.Event)
	}
	unchanged := prepare(context.Background(), changed.State, changedSrc, true)
	if unchanged.State.Frozen != changed.State.Frozen {
		t.Fatal("unchanged policy must keep the frozen prompt")
	}
	reorderedSrc := changedSrc
	reorderedSrc.Only = []string{"goal", "agents"}
	reorderedSrc.PolicySig = policySignature(reorderedSrc.Only)
	reordered := prepare(context.Background(), changed.State, reorderedSrc, true)
	if reordered.State.Frozen != changed.State.Frozen {
		t.Fatal("the same policy in a new order must not rebuild the prompt")
	}
}
