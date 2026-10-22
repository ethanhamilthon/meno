package engine

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"meno/internal/api"
	"meno/internal/llm"
)

// Several clients answer the same question at once: exactly one wins and the
// others get an explicit conflict, never a silent success.
func TestConcurrentAskAnswersHaveOneWinner(t *testing.T) {
	f := &fakeProvider{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	snap0, _ := e.Snapshot(context.Background(), id)
	done := make(chan error, 1)
	go func() {
		_, err := e.Resource(context.Background(), "POST", "/v1/sessions/"+id+"/ask", "", api.ResourceRequest{
			Token: snap0.AgentToken, Data: json.RawMessage(`{"form":{"questions":[{"id":"c","text":"Pick","type":"single","options":[{"id":"x","label":"X"},{"id":"y","label":"Y"}]}]}}`)})
		done <- err
	}()
	waitFor(t, e, id, func(s *runState) bool { return s.ask != nil })
	snap, _ := e.Snapshot(context.Background(), id)
	var wins, conflicts int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			opt := []string{`"x"`, `"y"`}[i%2]
			err := e.AnswerAsk(context.Background(), id, snap.Ask.AskID, api.AskAnswerRequest{Answers: map[string]json.RawMessage{"c": json.RawMessage(opt)}})
			switch {
			case err == nil:
				atomic.AddInt32(&wins, 1)
			case isCode(err, api.CodeConflict), isCode(err, api.CodeNotFound):
				atomic.AddInt32(&conflicts, 1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if wins != 1 || wins+conflicts != 8 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}
}
