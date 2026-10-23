package agent

import (
	"testing"

	"meno/internal/llm"
)

// NoAsk is stored with the session, survives a reload and is inherited by a fork.
func TestNoAskIsStoredAndForked(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession(t.TempDir(), "m")
	s.NoAsk = true
	s.Append(Turn{Role: llm.RoleUser, Text: "start"})
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	CloseSessionStores()
	got, err := LoadSession(home, s.ID)
	if err != nil || !got.NoAsk {
		t.Fatalf("reload: %+v %v", got, err)
	}
	f, _, err := ForkSession(home, s.ID, 1, "")
	if err != nil || !f.NoAsk {
		t.Fatalf("fork: %+v %v", f, err)
	}
}

// A session saved without the flag reads as false.
func TestNoAskDefaultsToFalse(t *testing.T) {
	home := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession(t.TempDir(), "m")
	s.Append(Turn{Role: llm.RoleUser, Text: "start"})
	if err := SaveSession(home, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession(home, s.ID)
	if err != nil || got.NoAsk {
		t.Fatalf("%+v %v", got, err)
	}
}
