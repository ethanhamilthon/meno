package app

import (
	"testing"
	"time"

	"meno/internal/client"
	"meno/internal/proto"
	"meno/internal/ui"
)

func TestUnattachedSessionCatalogDetailAndCountWithoutSnapshot(t *testing.T) {
	a := &App{
		cl:             &client.Client{},
		globalSessions: []proto.SessionItem{},
		rts:            map[string]*Runtime{},
	}
	unattached := proto.SessionItem{
		ID:        "unattached-1",
		Name:      "Inspect catalog",
		Cwd:       "/work/project",
		Provider:  "anthropic",
		Model:     "claude-3-7-sonnet",
		Effort:    "high",
		TurnCount: 3,
		UpdatedAt: time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}
	a.globalSessions = []proto.SessionItem{unattached}

	if a.rts[unattached.ID] != nil {
		t.Fatal("session must be unattached")
	}

	rows := a.sessionRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Turns != 3 {
		t.Fatalf("expected 3 turns, got %d", r.Turns)
	}
	expectedModel := "claude-3-7-sonnet (anthropic, effort high)"
	if r.Model != expectedModel {
		t.Fatalf("expected model %q, got %q", expectedModel, r.Model)
	}

	// Verify detail rendering
	detail := r.Cwd
	if r.Model != "" {
		detail = r.Model + " · " + r.Cwd
	}
	expectedDetail := expectedModel + " · /work/project"
	if detail != expectedDetail {
		t.Fatalf("expected detail %q, got %q", expectedDetail, detail)
	}

	turnsStr := itoa(r.Turns) + " turns"
	if turnsStr != "3 turns" {
		t.Fatalf("expected '3 turns', got %q", turnsStr)
	}
}

func TestUnattachedRowGlyphsAndFiltersWithAskingPrecedence(t *testing.T) {
	a, _ := queueTestApp()

	// 1. Running
	rowRunning := sessionRow{
		ID:      "run",
		running: true,
	}
	g, gfg := a.sessionGlyph(rowRunning)
	if g != "●" || (gfg != ui.Green && gfg != ui.FgDim) {
		t.Fatalf("running glyph mismatch: %q %v", g, gfg)
	}
	if !a.matchesFilter(rowRunning, filterRunning) || a.matchesFilter(rowRunning, filterAsking) || a.matchesFilter(rowRunning, filterDone) {
		t.Fatalf("running filter mismatch for rowRunning")
	}

	// 2. Asking (with running: true) -> Asking takes precedence over running!
	rowAsking := sessionRow{
		ID:       "ask",
		running:  true,
		needsYou: true,
	}
	g, gfg = a.sessionGlyph(rowAsking)
	if g != "?" || gfg != ui.Orange {
		t.Fatalf("asking glyph mismatch: got %q %v, want '?' %v", g, gfg, ui.Orange)
	}
	if !a.matchesFilter(rowAsking, filterAsking) {
		t.Fatal("rowAsking must match filterAsking")
	}
	if a.matchesFilter(rowAsking, filterRunning) {
		t.Fatal("asking session must not match filterRunning; asking takes precedence")
	}

	// 3. Unseen
	rowUnseen := sessionRow{
		ID:     "done",
		unseen: true,
	}
	g, gfg = a.sessionGlyph(rowUnseen)
	if g != "●" || gfg != ui.Blue {
		t.Fatalf("unseen glyph mismatch: got %q %v, want '●' %v", g, gfg, ui.Blue)
	}
	if !a.matchesFilter(rowUnseen, filterDone) || a.matchesFilter(rowUnseen, filterRunning) || a.matchesFilter(rowUnseen, filterAsking) {
		t.Fatal("unseen filter mismatch for rowUnseen")
	}

	// 4. Idle
	rowIdle := sessionRow{
		ID: "idle",
	}
	g, gfg = a.sessionGlyph(rowIdle)
	if g != "○" || gfg != ui.FgDim {
		t.Fatalf("idle glyph mismatch: got %q %v, want '○' %v", g, gfg, ui.FgDim)
	}
	if !a.matchesFilter(rowIdle, filterAll) || a.matchesFilter(rowIdle, filterRunning) || a.matchesFilter(rowIdle, filterAsking) || a.matchesFilter(rowIdle, filterDone) {
		t.Fatal("idle filter mismatch for rowIdle")
	}
}
