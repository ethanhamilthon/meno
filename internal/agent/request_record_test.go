package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"meno/internal/llm"
)

func TestRequestRecordsRoundTripConfigurationsAndFork(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession(dir, "current")
	s.Append(Turn{Role: "user", Text: "first"})
	req := llm.Request{Provider: llm.Provider{Name: "p", APIKey: "NEVER_EXPORT", BaseURL: "SECRET_ENDPOINT"}, Model: "m1", Effort: "low", System: "historical prompt", Tools: true, ToolName: "bash"}
	id := s.startRequest(req, "step1", 1, "conversation")
	s.requestUsage(id, llm.Usage{In: 100, Out: 10})
	s.endRequest(id, nil, false)
	s.Append(Turn{Role: "assistant", Text: "answer", Replay: &llm.Msg{Responses: []json.RawMessage{json.RawMessage(`{"secret":"OPAQUE_STATE"}`)}}})
	id = s.startRequest(req, "step2", 1, "conversation")
	s.endRequest(id, errors.New("failed"), false)
	req.Model, req.Effort, req.System = "m2", "high", "changed prompt"
	id = s.startRequest(req, "step2", 2, "conversation")
	s.requestUsage(id, llm.Usage{CacheReported: true})
	s.endRequest(id, nil, false)
	s.Append(Turn{Role: "assistant", Text: "second answer"})
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadExport(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Configurations) != 2 || len(got.Requests) != 3 {
		t.Fatalf("snapshot: %+v", got)
	}
	a, b, c := got.Requests[0], got.Requests[1], got.Requests[2]
	if a.Config != b.Config || a.Config == c.Config || b.Step != c.Step || c.Attempt != 2 || c.Effort != "high" || c.Model != "m2" {
		t.Fatal("historical request configuration changed")
	}
	if a.Usage == nil || a.Usage.CacheReported || b.Usage != nil || c.Usage == nil || !c.Usage.CacheReported {
		t.Fatal("unknown and zero accounting collapsed")
	}
	raw, _ := json.Marshal(got)
	for _, secret := range []string{"NEVER_EXPORT", "SECRET_ENDPOINT", "OPAQUE_STATE", "api_key"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("export leaked %s", secret)
		}
	}
	fork, _, err := ForkSession(dir, s.ID, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	rows, configs := fork.requestSnapshot()
	if len(rows) != 1 || len(configs) != 1 {
		t.Fatalf("fork copied future requests: %d %d", len(rows), len(configs))
	}
}

func TestRequestRecordsCompactionAndLegacy(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s, _ := compactFixture()
	s.Cwd = dir
	original := s.Len()
	cl := &prefixClient{failures: 1}
	changed := 0
	_, _, err := CompactWith(context.Background(), cl, llm.Provider{Name: "p"}, "m", s, CompactOptions{Keep: 4, System: "prompt", RecordChanged: func() { changed++ }})
	if err != nil {
		t.Fatal(err)
	}
	if changed != 4 {
		t.Fatalf("expected start/end for both summary attempts: %d", changed)
	}
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadExport(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Turns) != original || len(got.Requests) != 2 || got.Requests[0].Purpose != "compaction" || got.Requests[0].Outcome != "failed" || got.Requests[1].Usage != nil {
		t.Fatalf("compacted snapshot: %+v", got)
	}

	// A fork before the compaction boundary excludes those future attempts;
	// once a subsequent history turn exists, both compaction attempts survive.
	beforeFork, _, err := ForkSession(dir, s.ID, int64(original), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeFork.Requests) != 0 {
		t.Fatal("fork copied compaction from beyond its boundary")
	}
	s.Append(Turn{Role: "user", Text: "continue after compaction"})
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	afterFork, _, err := ForkSession(dir, s.ID, int64(original+1), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(afterFork.Requests) != 2 || afterFork.Requests[0].Record.Boundary != int64(original+1) {
		t.Fatal("compaction attempt ancestry lost")
	}
	s.Coverage, s.Legacy = CoveragePartial, true
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err = LoadExport(dir, s.ID)
	if err != nil || len(got.Warnings) != 2 {
		t.Fatalf("legacy warnings: %v %v", got.Warnings, err)
	}
}

type recordClient func(context.Context, llm.Request, func(llm.Event)) error

func (f recordClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	return f(ctx, req, emit)
}

func TestStreamAttemptRecordsMissingUsageAndCancellation(t *testing.T) {
	for _, reported := range []bool{false, true} {
		s := NewSession(t.TempDir(), "m")
		r := &Runner{D: Deps{Client: recordClient(func(_ context.Context, _ llm.Request, emit func(llm.Event)) error {
			emit(llm.Event{Kind: llm.EvDone, UsageReported: reported})
			return nil
		}), Emit: func(Ev) {}}}
		if _, _, err := r.streamAttempt(context.Background(), s, "step", 1); err != nil {
			t.Fatal(err)
		}
		rows, _ := s.requestSnapshot()
		if (rows[0].Record.Usage != nil) != reported || rows[0].Record.Outcome != "completed" {
			t.Fatal("zero and missing usage not distinguished")
		}
	}
	s := NewSession(t.TempDir(), "m")
	ctx, cancel := context.WithCancel(context.Background())
	r := &Runner{D: Deps{Client: recordClient(func(context.Context, llm.Request, func(llm.Event)) error { cancel(); return context.Canceled }), Emit: func(Ev) {}}}
	_, _, _ = r.streamAttempt(ctx, s, "step", 1)
	rows, _ := s.requestSnapshot()
	if rows[0].Record.Outcome != "cancelled" || rows[0].Record.Ended.Before(time.Now().Add(-time.Minute)) {
		t.Fatal("cancellation not finalized")
	}
}

func TestRequestRecordsTrackRealRetries(t *testing.T) {
	cl := &flaky{steps: []func(func(llm.Event)) error{
		failWith(&llm.HTTPError{Status: 503, Body: "unavailable"}),
		func(emit func(llm.Event)) error {
			emit(llm.Event{Kind: llm.EvText, Text: "done"})
			emit(llm.Event{Kind: llm.EvDone, UsageReported: true})
			return nil
		},
	}}
	var events []Ev
	runner := fastRunner(cl, &events)
	s := NewSession(t.TempDir(), "m")
	if err := runner.Turn(context.Background(), s, "hi"); err != nil {
		t.Fatal(err)
	}
	rows, configs := s.requestSnapshot()
	if len(rows) != 2 || len(configs) != 1 {
		t.Fatalf("attempt count: %d configurations: %d", len(rows), len(configs))
	}
	a, b := rows[0].Record, rows[1].Record
	if a.ID == b.ID || a.Step != b.Step || a.Attempt != 1 || b.Attempt != 2 || a.Outcome != "failed" || b.Outcome != "completed" || a.Usage != nil || b.Usage == nil {
		t.Fatalf("retry records: %+v %+v", a, b)
	}
}
