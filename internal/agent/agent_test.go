package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meno/internal/jsvm"
	"meno/internal/llm"
)

type scripted struct {
	steps [][]llm.Event
	calls int
	block bool
}

func (s *scripted) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	i := s.calls
	s.calls++
	if s.block {
		<-ctx.Done()
		return ctx.Err()
	}
	if i >= len(s.steps) {
		return errors.New("no more steps")
	}
	for _, e := range s.steps[i] {
		emit(e)
	}
	return nil
}

func newRunner(cl llm.Client, evs *[]Ev) *Runner {
	vm := jsvm.New(jsvm.Options{Root: ".", Timeout: 2 * time.Second})
	return &Runner{D: Deps{
		Client: cl, Provider: llm.Provider{Name: "t"}, Model: "m", VM: vm,
		Thinking: true, Emit: func(e Ev) { *evs = append(*evs, e) },
	}}
}

func TestTurnWithJSThenAnswer(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{
		{{Kind: llm.EvText, Text: "checking"}, {Kind: llm.EvTool, CallID: "c1", Code: "console.log(2*21)"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "answer 42"}, {Kind: llm.EvDone}},
	}}
	var evs []Ev
	r := newRunner(cl, &evs)
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "q"); err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshot()) != 3 {
		t.Fatalf("turns %d: %+v", len(s.Snapshot()), s.Snapshot())
	}
	if s.Snapshot()[1].Code != "console.log(2*21)" || !strings.Contains(s.Snapshot()[1].Output, "42") || s.Snapshot()[1].Text != "checking" {
		t.Fatalf("js turn: %+v", s.Snapshot()[1])
	}
	if s.Snapshot()[2].Text != "answer 42" {
		t.Fatalf("final: %+v", s.Snapshot()[2])
	}
	kinds := map[string]int{}
	for _, e := range evs {
		kinds[e.Kind]++
	}
	if kinds[EvJSStart] != 1 || kinds[EvJSEnd] != 1 || kinds[EvDone] != 1 {
		t.Fatalf("events %v", kinds)
	}
}

func TestJSErrorIsFedBack(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "throw new Error('bad')"}},
		{{Kind: llm.EvText, Text: "sorry"}, {Kind: llm.EvDone}},
	}}
	var evs []Ev
	r := newRunner(cl, &evs)
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "q"); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot()[1].Err == "" {
		t.Fatalf("error not recorded: %+v", s.Snapshot()[1])
	}
	msgs := s.ToMsgs()
	found := false
	for _, m := range msgs {
		if m.Role == llm.RoleTool && strings.Contains(m.Text, "ERROR") {
			found = true
		}
	}
	if !found {
		t.Fatal("tool error not passed back to model")
	}
}

func TestNoStepLimit(t *testing.T) {
	var steps [][]llm.Event
	for i := 0; i < 60; i++ {
		steps = append(steps, []llm.Event{{Kind: llm.EvTool, CallID: "c", Code: "1"}})
	}
	steps = append(steps, []llm.Event{{Kind: llm.EvText, Text: "done"}})
	var evs []Ev
	r := newRunner(&scripted{steps: steps}, &evs)
	if err := r.Turn(context.Background(), NewSession(".", "m"), "q"); err != nil {
		t.Fatalf("long tool loop must not hit a limit: %v", err)
	}
	if evs[len(evs)-1].Kind != EvDone {
		t.Fatalf("last event %+v", evs[len(evs)-1])
	}
}

func TestAbortMarksSession(t *testing.T) {
	var evs []Ev
	r := newRunner(&scripted{block: true}, &evs)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	s := NewSession(".", "m")
	err := r.Turn(ctx, s, "q")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
	if s.Snapshot()[len(s.Snapshot())-1].Text != "[aborted]" {
		t.Fatalf("abort not recorded")
	}
}

func TestStreamErrorSurfaces(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{{{Kind: llm.EvError, Text: "rate limited"}}}}
	var evs []Ev
	r := newRunner(cl, &evs)
	err := r.Turn(context.Background(), NewSession(".", "m"), "q")
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("got %v", err)
	}
}

func TestSessionSaveLoadList(t *testing.T) {
	dir := t.TempDir()
	s := NewSession(".", "m")
	s.Replace([]Turn{{Role: llm.RoleUser, Text: "check routes"}, {Role: llm.RoleAssistant, Code: "1", Output: "1"}})
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession(dir, s.ID)
	if err != nil || got.Title != "check routes" || len(got.Turns) != 2 {
		t.Fatalf("load: %v %+v", err, got)
	}
	list, err := ListSessions(dir)
	if err != nil || len(list) != 1 || list[0].JSRuns != 1 {
		t.Fatalf("list: %v %+v", err, list)
	}
}

func TestListSessionsMissingDir(t *testing.T) {
	list, err := ListSessions(filepath.Join(t.TempDir(), "none"))
	if err != nil || list != nil {
		t.Fatalf("missing dir: %v %v", list, err)
	}
}

func TestSaveRejectsEmptyID(t *testing.T) {
	if err := SaveSession(t.TempDir(), &Session{}); err == nil {
		t.Fatal("empty id accepted")
	}
}

func TestTitleTruncatesLongRunes(t *testing.T) {
	s := &Session{Turns: []Turn{{Role: llm.RoleUser, Text: strings.Repeat("é", 100)}}}
	if got := deriveTitle(s); len([]rune(got)) != 60 {
		t.Fatalf("len %d", len([]rune(got)))
	}
}

func TestCompactKeepsTail(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "SUMMARY"}}}}
	s := NewSession(".", "m")
	for i := 0; i < 10; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: strings.Repeat("x", 400)})
	}
	before, after, err := Compact(context.Background(), cl, llm.Provider{}, "m", s)
	if err != nil || len(s.Snapshot()) != keepTail || s.Summary != "SUMMARY" || after >= before {
		t.Fatalf("compact: %v before=%d after=%d turns=%d", err, before, after, len(s.Snapshot()))
	}
	if msgs := s.ToMsgs(); msgs[0].Role != llm.RoleUser || !strings.Contains(msgs[0].Text, "SUMMARY") {
		t.Fatal("summary not injected")
	}
}
