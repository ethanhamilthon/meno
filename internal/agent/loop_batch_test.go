package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"meno/internal/llm"
)

func twoCalls(a, b string) [][]llm.Event {
	return [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: a}, {Kind: llm.EvTool, CallID: "c2", Code: b}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}},
	}
}

// Both calls wait for each other's file: they finish only when they run at once.
func TestBatchCallsStartTogetherAndResultsKeepCallOrder(t *testing.T) {
	dir := t.TempDir()
	a := "touch a; while [ ! -e b ]; do sleep 0.01; done; sleep 0.2; echo first"
	b := "touch b; while [ ! -e a ]; do sleep 0.01; done; echo second"
	var evs []Ev
	r := newRunner(&scripted{steps: twoCalls(a, b)}, &evs)
	r.D.Tool = newShellTool(dir)
	s := NewSession(dir, "m")
	if err := r.Turn(context.Background(), s, "go"); err != nil {
		t.Fatal(err)
	}
	turns := s.Snapshot()
	if len(turns) < 3 || turns[1].CallID != "c1" || turns[2].CallID != "c2" || turns[1].Output != "first" || turns[2].Output != "second" {
		t.Fatalf("%+v", turns)
	}
	if turns[1].ReplayGroup == "" || turns[1].ReplayGroup != turns[2].ReplayGroup {
		t.Fatal("one replay group for the batch")
	}
	var ends []string
	for _, e := range evs {
		if e.Kind == EvJSEnd {
			ends = append(ends, e.CallID)
		}
	}
	if strings.Join(ends, ",") != "c2,c1" {
		t.Fatalf("c2 finishes first, results stay in call order: %v", ends)
	}
}

func TestBatchFailureDoesNotCancelSiblings(t *testing.T) {
	var evs []Ev
	r := newRunner(&scripted{steps: twoCalls("exit 3", "sleep 0.2; echo fine")}, &evs)
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "go"); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot()[2].Output; got != "fine" {
		t.Fatalf("sibling output %q", got)
	}
}

func TestHumanMessagePromotesRunningCallsAndTheModelContinues(t *testing.T) {
	dir := t.TempDir()
	var evs []Ev
	cl := &scripted{steps: twoCalls("echo quick", "sleep 30")}
	r := newRunner(cl, &evs)
	r.D.Tool = newShellTool(dir)
	r.D.Queue = &Queue{}
	s := NewSession(dir, "m")
	go func() {
		time.Sleep(300 * time.Millisecond)
		r.D.Queue.Push(Message{Text: "hurry"})
	}()
	start := time.Now()
	if err := r.Turn(context.Background(), s, "go"); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("the model waited for the promoted call")
	}
	turns := s.Snapshot()
	if turns[1].Output != "quick" || !strings.Contains(turns[2].Output, "background task") || !strings.Contains(turns[2].Output, "meno tasks logs") {
		t.Fatalf("%+v", turns)
	}
	var promoted int
	for _, e := range evs {
		if e.Kind == EvJSEnd && e.TaskID != "" {
			promoted++
		}
	}
	if promoted != 1 {
		t.Fatalf("promoted %d calls", promoted)
	}
	last := turns[len(turns)-1]
	if last.Role != llm.RoleAssistant || last.Text != "ok" {
		t.Fatalf("the model was not called again: %+v", turns)
	}
	hasHuman := false
	for _, tn := range turns {
		hasHuman = hasHuman || tn.Role == llm.RoleUser && tn.Text == "hurry"
	}
	if !hasHuman {
		t.Fatal("the human message was not delivered")
	}
}

func TestNonHumanEventsNeverPromote(t *testing.T) {
	dir := t.TempDir()
	var evs []Ev
	r := newRunner(&scripted{steps: twoCalls("sleep 0.4; echo a", "echo b")}, &evs)
	r.D.Tool = newShellTool(dir)
	r.D.Queue = &Queue{}
	r.D.Queue.Push(Message{Viewed: true, Text: "image"}) // not a human message
	s := NewSession(dir, "m")
	if err := r.Turn(context.Background(), s, "go"); err != nil {
		t.Fatal(err)
	}
	got := ""
	for _, tn := range s.Snapshot() {
		if tn.CallID == "c1" {
			got = tn.Output
		}
	}
	if got != "a" {
		t.Fatalf("a non-human input interrupted the call: %q", got)
	}
}

func TestStopCancelsForegroundCallsAndEveryCallGetsOneResult(t *testing.T) {
	dir := t.TempDir()
	var evs []Ev
	r := newRunner(&scripted{steps: twoCalls("sleep 30", "sleep 30")}, &evs)
	r.D.Tool = newShellTool(dir)
	s := NewSession(dir, "m")
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	start := time.Now()
	err := r.Turn(ctx, s, "go")
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("stop: %v after %v", err, time.Since(start))
	}
	ids := map[string]int{}
	for _, tn := range s.Snapshot() {
		if tn.CallID != "" {
			ids[tn.CallID]++
		}
	}
	if ids["c1"] != 1 || ids["c2"] != 1 {
		t.Fatalf("calls need exactly one result: %v", ids)
	}
}
