package app

import (
	"strings"
	"testing"

	"meno/internal/agent"
)

// A session is named after the first user message until the user renames it.
func TestRenameCommand(t *testing.T) {
	a, _ := newTestApp(t)
	a.st.Session.Append(agent.Turn{Role: "user", Text: "remember the milk"})
	if got := a.st.Session.DisplayTitle(); got != "remember the milk" {
		t.Fatalf("default title: %q", got)
	}
	a.runCommand("rename Buy milk soon")
	if a.st.Session.Title != "Buy milk soon" {
		t.Fatalf("title: %q", a.st.Session.Title)
	}
	a.runCommand("rename " + strings.Repeat("x", agent.MaxTitleRunes+1))
	if a.st.Session.Title != "Buy milk soon" {
		t.Fatalf("a too long name was accepted: %q", a.st.Session.Title)
	}
}
