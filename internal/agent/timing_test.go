package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"meno/internal/llm"
)

func lastTurn(s *Session) Turn {
	t := s.Snapshot()
	return t[len(t)-1]
}

func TestJSCallRecordsDuration(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "sleep 0.1"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}},
	}}
	var evs []Ev
	r := newRunner(cl, &evs)
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "q"); err != nil {
		t.Fatal(err)
	}
	call := s.Snapshot()[1]
	if call.ElapsedMs < 90 || call.ElapsedMs > 1500 {
		t.Fatalf("elapsed %dms", call.ElapsedMs)
	}
	var start, end *Ev
	for i := range evs {
		switch evs[i].Kind {
		case EvJSStart:
			start = &evs[i]
		case EvJSEnd:
			end = &evs[i]
		}
	}
	if start == nil || end == nil || end.Elapsed < 90*time.Millisecond {
		t.Fatalf("events: %+v %+v", start, end)
	}
}

func TestFastCallIsStillTimed(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "true"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}},
	}}
	var evs []Ev
	s := NewSession(".", "m")
	if err := newRunner(cl, &evs).Turn(context.Background(), s, "q"); err != nil {
		t.Fatal(err)
	}
	// 0 means "not timed" (old sessions); a call that took microseconds still counts.
	if got := s.Snapshot()[1].ElapsedMs; got < 1 {
		t.Fatalf("elapsed %d", got)
	}
}

func usageEv(in, out, cached int) llm.Event {
	return llm.Event{Kind: llm.EvDone, Usage: llm.Usage{In: in, Out: out, Cached: cached}}
}

func TestRunStatsOfFinishedRun(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "sleep 0.05"}, usageEv(100, 10, 50)},
		{{Kind: llm.EvTool, CallID: "c2", Code: "true"}, usageEv(200, 20, 150)},
		{{Kind: llm.EvText, Text: "done"}, usageEv(300, 5, 250)},
	}}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.Cost = func(u llm.Usage) (float64, bool) { return float64(u.In) / 1000, true }
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "q"); err != nil {
		t.Fatal(err)
	}
	turns := s.Snapshot()
	for i, tr := range turns[:len(turns)-1] {
		if tr.Run != nil {
			t.Fatalf("turn %d carries the run summary", i)
		}
	}
	run := turns[len(turns)-1].Run
	if run == nil {
		t.Fatal("no run stats on the last turn")
	}
	if run.Status != RunOK || run.Turns != 3 || run.ToolCalls != 2 || run.Err != "" {
		t.Fatalf("%+v", run)
	}
	if run.ToolMs < 40 || run.ElapsedMs < run.ToolMs {
		t.Fatalf("tool %dms of %dms", run.ToolMs, run.ElapsedMs)
	}
	if run.Usage != (llm.Usage{In: 600, Out: 35, Cached: 450}) {
		t.Fatalf("usage %+v", run.Usage)
	}
	if run.Cost == nil || *run.Cost < 0.599 || *run.Cost > 0.601 {
		t.Fatalf("cost %v", run.Cost)
	}
}

func TestRunStatsWithoutPriceHaveNoCost(t *testing.T) {
	for name, cost := range map[string]func(llm.Usage) (float64, bool){
		"no pricer":  nil,
		"no price":   func(llm.Usage) (float64, bool) { return 0, false },
		"half known": nil,
	} {
		t.Run(name, func(t *testing.T) {
			cl := &scripted{steps: [][]llm.Event{
				{{Kind: llm.EvTool, CallID: "c1", Code: "true"}, usageEv(10, 1, 0)},
				{{Kind: llm.EvText, Text: "done"}, usageEv(20, 1, 0)},
			}}
			var evs []Ev
			r := newRunner(cl, &evs)
			r.D.Cost = cost
			if name == "half known" {
				n := 0
				r.D.Cost = func(llm.Usage) (float64, bool) { n++; return 1, n == 1 }
			}
			s := NewSession(".", "m")
			if err := r.Turn(context.Background(), s, "q"); err != nil {
				t.Fatal(err)
			}
			if run := lastTurn(s).Run; run == nil || run.Turns != 2 || (name != "half known" && run.Cost != nil) || (name == "half known" && (run.Cost == nil || !run.Partial)) {
				t.Fatalf("%+v", run)
			}
		})
	}
}

func TestRunStatsOfFailedRun(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "true"}, usageEv(10, 1, 0)},
		{{Kind: llm.EvError, Text: "http 503:\n  service   unavailable"}},
	}}
	var evs []Ev
	s := NewSession(".", "m")
	err := newRunner(cl, &evs).Turn(context.Background(), s, "q")
	if err == nil {
		t.Fatal("want an error")
	}
	run := lastTurn(s).Run
	if run == nil || run.Status != RunFail || run.Turns != 1 || run.ToolCalls != 1 {
		t.Fatalf("%+v", run)
	}
	if run.Err != "http 503: service unavailable" {
		t.Fatalf("error not squeezed into a line: %q", run.Err)
	}
}

func TestRunStatsOfFailureBeforeAnyAnswer(t *testing.T) {
	var evs []Ev
	s := NewSession(".", "m")
	if err := newRunner(&scripted{}, &evs).Turn(context.Background(), s, "q"); err == nil {
		t.Fatal("want an error")
	}
	// Only the user turn exists: it carries the summary.
	if last := lastTurn(s); last.Role != llm.RoleUser || last.Run == nil || last.Run.Status != RunFail || last.Run.Turns != 0 {
		t.Fatalf("%+v", last)
	}
}

func TestRunStatsOfStoppedRun(t *testing.T) {
	var evs []Ev
	r := newRunner(&scripted{block: true}, &evs)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	s := NewSession(".", "m")
	if err := r.Turn(ctx, s, "q"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	last := lastTurn(s)
	if last.Text != "[aborted]" || last.Run == nil || last.Run.Status != RunStop || last.Run.ElapsedMs < 20 {
		t.Fatalf("%+v %+v", last, last.Run)
	}
}
