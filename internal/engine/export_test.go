package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/proto"
)

func readExport(t *testing.T, e *Engine, id string) agent.ExportSnapshot {
	t.Helper()
	r, err := e.Resource(context.Background(), "GET", "/v1/sessions/"+id+"/export", "", proto.ResourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var response proto.ExportResponse
	if err := json.Unmarshal(r.Data, &response); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response.Markdown, "Session ID: "+id) || !strings.Contains(response.Markdown, "## Model requests") {
		t.Fatal("resource did not return rendered session Markdown")
	}
	out, err := e.exportSnapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestExportWhileStreamingAndAfterCancellation(t *testing.T) {
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	if _, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "hi", ClientID: "one"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.seen:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	snapshot := readExport(t, e, id)
	if !snapshot.Running || len(snapshot.Requests) != 1 || snapshot.Requests[0].Outcome != "pending" || snapshot.Requests[0].Usage != nil {
		t.Fatalf("pending export: %+v", snapshot)
	}
	if len(snapshot.Configurations) != 1 || len(snapshot.Warnings) == 0 || len(snapshot.Turns) != 1 {
		t.Fatal("incomplete cutoff not represented")
	}
	if err := e.Stop(context.Background(), id, proto.StopRequest{}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	completed := readExport(t, e, id)
	if completed.Running || completed.Requests[0].Outcome != "cancelled" {
		t.Fatalf("cancelled export: %+v", completed)
	}
	if snapshot.Requests[0].Outcome != "pending" {
		t.Fatal("previous snapshot mutated")
	}
}

func TestExportCompletedRequestSurvivesReloadAndSettingsChange(t *testing.T) {
	f := &fakeProvider{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "answer"}, {Kind: llm.EvDone, UsageReported: true, Usage: llm.Usage{In: 9, Out: 2, CacheReported: true}}}}}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	if _, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "hi", ClientID: "one"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	before := readExport(t, e, id)
	if before.Requests[0].Usage == nil || before.Requests[0].Usage.In != 9 || before.Requests[0].Outcome != "completed" {
		t.Fatalf("completed export: %+v", before)
	}
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	s.session.Model, s.session.Effort = "changed", "high"
	if err := e.save(s); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	e.mu.Lock()
	delete(e.sessions, id)
	e.mu.Unlock()
	after := readExport(t, e, id)
	if after.Model != "changed" || after.Requests[0].Model != "fake" || after.Requests[0].Config != before.Requests[0].Config {
		t.Fatal("current settings rewrote history")
	}
	if len(after.Turns) != 2 || after.Turns[1].Text != "answer" {
		t.Fatal("transcript missing after reload")
	}
	r, err := e.Resource(context.Background(), "GET", "/v1/sessions/"+id+"/export", "", proto.ResourceRequest{Token: "wrong"})
	if err == nil || len(r.Data) != 0 {
		t.Fatal("agent token received human export")
	}
	_, err = e.Resource(context.Background(), "GET", "/v1/sessions/missing/export", "", proto.ResourceRequest{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing session: %v", err)
	}
}

func TestExportEmptyUnsavedSession(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	snapshot := readExport(t, e, id)
	if snapshot.ID != id || len(snapshot.Turns) != 0 {
		t.Fatalf("empty export: %+v", snapshot)
	}
}

func TestExportInterruptedAttemptAndLegacyWarning(t *testing.T) {
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	if _, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "hi", ClientID: "one"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.seen:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	// Read the durable pending attempt into an independently loaded session,
	// as a new daemon would do after the old process exits mid-stream.
	saved, err := agent.LoadSession(e.home, id)
	if err != nil {
		t.Fatal(err)
	}
	saved.ID = agent.NewID()
	state, err := e.newState(saved, false)
	if err != nil {
		t.Fatal(err)
	}
	state.mu.Lock()
	err = e.ensureSaved(state)
	state.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	e.sessions[saved.ID] = state
	e.mu.Unlock()
	out := readExport(t, e, saved.ID)
	if out.Running || out.Requests[0].Outcome != "interrupted: final outcome unavailable" || out.Requests[0].Usage != nil {
		t.Fatalf("interrupted export: %+v", out)
	}
	legacy := agent.NewSession(t.TempDir(), "old")
	legacy.Coverage = agent.CoveragePartial
	legacy.Append(agent.Turn{Role: "user", Text: "retained tail"})
	if err := agent.SaveSession(e.home, legacy); err != nil {
		t.Fatal(err)
	}
	out = readExport(t, e, legacy.ID)
	if len(out.Warnings) != 2 || len(out.Requests) != 0 {
		t.Fatalf("legacy export: %+v", out)
	}
}
