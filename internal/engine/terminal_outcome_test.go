package engine

import (
	"context"
	"meno/internal/api"
	"meno/internal/llm"
	"testing"
)

type failingClient struct{}

func (failingClient) Stream(context.Context, llm.Request, func(llm.Event)) error {
	return &llm.HTTPError{Status: 400, Body: "bad request"}
}

// A provider failure must end the run with an error event, never with done.
func TestProviderErrorIsNotReportedAsDone(t *testing.T) {
	e, h := fixture(t, &fakeProvider{})
	e.client = func(llm.Provider) llm.Client { return failingClient{} }
	id := newSession(t, e)
	if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: "hi", ClientID: "m"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running && s.lastErr != "" })
	ch, stop, _ := h.Subscribe(id, 0, "boot")
	defer stop()
	var gotErr, gotDone bool
	for len(ch) > 0 {
		ev := <-ch
		gotErr = gotErr || ev.Type == api.EventError
		gotDone = gotDone || ev.Type == api.EventDone
	}
	if !gotErr || gotDone {
		t.Fatalf("error=%v done=%v", gotErr, gotDone)
	}
}
