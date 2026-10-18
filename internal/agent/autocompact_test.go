package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"meno/internal/llm"
)

// sizedClient answers summary requests with a short text and normal requests
// with "ok"; it records which kind it saw.
type sizedClient struct {
	summaries, normals int
	failSummary        bool
}

func (c *sizedClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	if last := req.Msgs[len(req.Msgs)-1].Text; strings.Contains(last, "Summarize this conversation") {
		c.summaries++
		if c.failSummary {
			return errors.New("summary model down")
		}
		emit(llm.Event{Kind: llm.EvText, Text: "## Goal\nkeep going"})
	} else {
		c.normals++
		emit(llm.Event{Kind: llm.EvText, Text: "ok"})
	}
	emit(llm.Event{Kind: llm.EvDone, Usage: llm.Usage{In: 100}})
	return nil
}

func bigSession(n int) *Session {
	s := NewSession(".", "m")
	for i := 0; i < n; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: strings.Repeat("x", 400)})
		s.Append(Turn{Role: llm.RoleAssistant, Text: strings.Repeat("y", 400)})
	}
	return s
}

func TestAutoCompactTriggersAtThresholdAndKeepsHistory(t *testing.T) {
	cl := &sizedClient{}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.AutoCompact = AutoCompact{Window: 4000, Percent: 80, Keep: 8}
	s := bigSession(20) // ~8000 chars of messages = ~2000+ tokens... plus allowance
	s.Append(Turn{Role: llm.RoleUser, Text: "continue"})
	before := s.Len()
	if err := r.Continue(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if cl.summaries != 1 || cl.normals != 1 {
		t.Fatalf("summaries %d normal requests %d", cl.summaries, cl.normals)
	}
	if s.Len() >= before || s.Summary == "" {
		t.Fatalf("context not reduced: %d -> %d", before, s.Len())
	}
	p := s.ensureLog().Pending()
	if len(p.Archive) == 0 {
		t.Fatal("compaction must move the originals to the history, not drop them")
	}
}

func TestAutoCompactOffWithoutKnownWindowAndBelowThreshold(t *testing.T) {
	cl := &sizedClient{}
	var evs []Ev
	r := newRunner(cl, &evs)
	s := bigSession(20)
	s.Append(Turn{Role: llm.RoleUser, Text: "go"})
	if err := r.Continue(context.Background(), s); err != nil || cl.summaries != 0 {
		t.Fatalf("an unknown window must never trigger: %v %d", err, cl.summaries)
	}
	cl2 := &sizedClient{}
	r2 := newRunner(cl2, &evs)
	r2.D.AutoCompact = AutoCompact{Window: 1_000_000}
	s2 := bigSession(20)
	s2.Append(Turn{Role: llm.RoleUser, Text: "go"})
	if err := r2.Continue(context.Background(), s2); err != nil || cl2.summaries != 0 {
		t.Fatalf("below the threshold: %v %d", err, cl2.summaries)
	}
}

func TestAutoCompactFailureKeepsContextAndNoLoopOnSameContext(t *testing.T) {
	cl := &sizedClient{failSummary: true}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.AutoCompact = AutoCompact{Window: 4000, Percent: 50, Keep: 8, OutputReserve: 100}
	s := bigSession(20)
	s.Append(Turn{Role: llm.RoleUser, Text: "go"})
	before := s.Len()
	err := r.Continue(context.Background(), s)
	if !errors.Is(err, ErrNoRoom) {
		t.Fatalf("a request that cannot fit and cannot be compacted must say so: %v", err)
	}
	if s.Len() < before || cl.summaries != 1 {
		t.Fatalf("failed compaction must keep the old context: %d -> %d, %d attempts", before, s.Len(), cl.summaries)
	}
	if cl.normals != 0 {
		t.Fatal("no request may be sent when it cannot fit")
	}
}
