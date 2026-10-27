package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"meno/internal/db"
	"meno/internal/llm"
)

func TestStoppedInsideJSStillCountsTheCall(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "while :; do :; done"}, usageEv(1, 1, 0)},
	}}
	var evs []Ev
	vm := newShellTool(".")
	r := newRunner(cl, &evs)
	r.D.Tool = vm
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(80 * time.Millisecond); cancel() }()
	s := NewSession(".", "m")
	if err := r.Turn(ctx, s, "q"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var call Turn
	for _, tr := range s.Snapshot() {
		if tr.CallID == "c1" {
			call = tr
		}
	}
	if call.ElapsedMs < 50 || call.TimedOut || call.LimitMs != 0 {
		t.Fatalf("%+v", call)
	}
	if run := lastTurn(s).Run; run == nil || run.Status != RunStop || run.ToolCalls != 1 {
		t.Fatalf("%+v", run)
	}
}

func TestRunsDoNotLeakIntoEachOther(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{
		{{Kind: llm.EvText, Text: "one"}, usageEv(10, 1, 0)},
		{{Kind: llm.EvText, Text: "two"}, usageEv(20, 2, 0)},
	}}
	var evs []Ev
	r := newRunner(cl, &evs)
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Turn(context.Background(), s, "b"); err != nil {
		t.Fatal(err)
	}
	var runs []*RunStats
	for _, tr := range s.Snapshot() {
		if tr.Run != nil {
			runs = append(runs, tr.Run)
		}
	}
	if len(runs) != 2 || runs[0].Usage.In != 10 || runs[1].Usage.In != 20 || runs[1].Turns != 1 {
		t.Fatalf("%+v %+v", runs[0], runs[1])
	}
}

func TestTimingAndRunSurviveSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(CloseSessionStores)
	cost := 0.0123
	s := NewSession("/w", "m")
	s.Append(Turn{Role: llm.RoleUser, Text: "q", Time: time.Now()})
	s.Append(Turn{Role: llm.RoleAssistant, CallID: "c1", Code: "x", Err: "js timeout after 30s", ElapsedMs: 30012, LimitMs: 30000, TimedOut: true, Time: time.Now()})
	s.Append(Turn{Role: llm.RoleAssistant, Text: "bye", Time: time.Now()})
	s.SetRun(RunStats{Status: RunFail, ElapsedMs: 31000, Turns: 2, ToolCalls: 1, ToolMs: 30012, Usage: llm.Usage{In: 5, Out: 6, Cached: 7, CacheWrite: 8}, Cost: &cost, Err: "boom"})
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	turns := got.Snapshot()
	if c := turns[1]; c.ElapsedMs != 30012 || c.LimitMs != 30000 || !c.TimedOut || c.Run != nil {
		t.Fatalf("call turn %+v", c)
	}
	run := turns[2].Run
	if run == nil || run.Status != RunFail || run.Turns != 2 || run.ToolMs != 30012 || run.Usage.CacheWrite != 8 || run.Cost == nil || *run.Cost != cost || run.Err != "boom" {
		t.Fatalf("run %+v", run)
	}
	if turns[0].Run != nil || turns[0].ElapsedMs != 0 || turns[0].TimedOut {
		t.Fatalf("user turn %+v", turns[0])
	}
}

func TestDamagedRunSummaryDoesNotBreakLoading(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(CloseSessionStores)
	s := NewSession("/w", "m")
	s.Append(Turn{Role: llm.RoleAssistant, Text: "a", Time: time.Now()})
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	d, _ := db.Open(dir)
	if _, err := d.Exec("UPDATE turns SET run = '{not json'"); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession(dir, s.ID)
	if err != nil || len(got.Snapshot()) != 1 || got.Snapshot()[0].Run != nil || !strings.Contains(got.Snapshot()[0].Text, "a") {
		t.Fatalf("%v %+v", err, got)
	}
}
