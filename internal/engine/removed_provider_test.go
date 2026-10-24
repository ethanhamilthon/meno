package engine

import (
	"context"
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/llm"
)

func TestRemovedAccountProviderPreservesStoredHistory(t *testing.T) {
	st := config.DefaultSettings()
	st.Provider, st.Model = "json", "jm"
	e := jsonOnlyEngine(t, done(), st, provider("json", false))
	id := emptySession(t, e, "oauth", "old-model")
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "earlier conversation"})
	err = e.save(s)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Message(context.Background(), id, api.MessageRequest{Text: "continue", ClientID: "removed-provider"})
	if err == nil || !strings.Contains(err.Error(), `Provider "oauth" is not configured. Choose another with /model.`) {
		t.Fatalf("error: %v", err)
	}
	stored, err := agent.LoadSession(e.home, id)
	if err != nil {
		t.Fatal(err)
	}
	turns := stored.Snapshot()
	if stored.Provider != "oauth" || stored.Model != "old-model" || len(turns) != 1 || turns[0].Text != "earlier conversation" {
		t.Fatalf("stored session changed: %+v, turns: %+v", stored, turns)
	}
}
