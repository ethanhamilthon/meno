package agent

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meno/internal/jsvm"
	"meno/internal/llm"
)

// flaky answers each request with the next function.
type flaky struct {
	steps []func(emit func(llm.Event)) error
	calls int
}

func (f *flaky) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	i := f.calls
	f.calls++
	if i >= len(f.steps) {
		return errors.New("unexpected request")
	}
	return f.steps[i](emit)
}

func okText(text string) func(func(llm.Event)) error {
	return func(emit func(llm.Event)) error {
		emit(llm.Event{Kind: llm.EvText, Text: text})
		emit(llm.Event{Kind: llm.EvDone})
		return nil
	}
}

func failWith(err error) func(func(llm.Event)) error {
	return func(func(llm.Event)) error { return err }
}

func fastRunner(cl llm.Client, evs *[]Ev) *Runner {
	r := newRunner(cl, evs)
	r.D.Retry = RetryPolicy{Base: time.Millisecond, Cap: 2 * time.Millisecond}
	return r
}

func retries(evs []Ev) (out []Ev) {
	for _, e := range evs {
		if e.Kind == EvRetry {
			out = append(out, e)
		}
	}
	return
}

func TestRetriesTransientErrorsThenSucceeds(t *testing.T) {
	cl := &flaky{steps: []func(func(llm.Event)) error{
		failWith(&llm.HTTPError{Status: 503, Body: "overloaded"}),
		failWith(&llm.HTTPError{Status: 429, Body: "slow down"}),
		okText("hello"),
	}}
	var evs []Ev
	r := fastRunner(cl, &evs)
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "hi"); err != nil {
		t.Fatal(err)
	}
	rs := retries(evs)
	if cl.calls != 3 || len(rs) != 2 || rs[0].Attempt != 2 || rs[1].Attempt != 3 || rs[0].Max != 5 || !strings.Contains(rs[0].Err, "503") {
		t.Fatalf("calls %d retries %+v", cl.calls, rs)
	}
	snap := s.Snapshot()
	if len(snap) != 2 || snap[1].Text != "hello" {
		t.Fatalf("session %+v", snap)
	}
}

func TestMidStreamDropRestartsAndRunsToolOnce(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	bump := "sh('echo x >> " + counter + "')"
	drop := func(emit func(llm.Event)) error {
		emit(llm.Event{Kind: llm.EvText, Text: "par"})
		emit(llm.Event{Kind: llm.EvTool, CallID: "c1", Code: bump})
		return io.ErrUnexpectedEOF
	}
	good := func(emit func(llm.Event)) error {
		emit(llm.Event{Kind: llm.EvText, Text: "full"})
		emit(llm.Event{Kind: llm.EvTool, CallID: "c2", Code: bump})
		emit(llm.Event{Kind: llm.EvDone})
		return nil
	}
	cl := &flaky{steps: []func(func(llm.Event)) error{drop, good, okText("done")}}
	var evs []Ev
	r := fastRunner(cl, &evs)
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "go"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(counter); string(b) != "x\n" {
		t.Fatalf("tool executed %d times", len(b)/2)
	}
	for _, turn := range s.Snapshot() {
		if strings.Contains(turn.Text, "par") && !strings.Contains(turn.Text, "full") {
			t.Fatalf("partial output of the dropped request survived: %+v", turn)
		}
	}
}

func TestPermanentErrorsAreNotRetried(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 422} {
		cl := &flaky{steps: []func(func(llm.Event)) error{failWith(&llm.HTTPError{Status: status, Body: "no"})}}
		var evs []Ev
		r := fastRunner(cl, &evs)
		err := r.Turn(context.Background(), NewSession(".", "m"), "hi")
		if err == nil || cl.calls != 1 || len(retries(evs)) != 0 {
			t.Fatalf("status %d: err=%v calls=%d", status, err, cl.calls)
		}
	}
}

func TestGivesUpAfterFiveAttemptsAndKeepsTheMessage(t *testing.T) {
	boom := failWith(&llm.HTTPError{Status: 500, Body: "down"})
	cl := &flaky{steps: []func(func(llm.Event)) error{boom, boom, boom, boom, boom, okText("never")}}
	var evs []Ev
	r := fastRunner(cl, &evs)
	s := NewSession(".", "m")
	err := r.Turn(context.Background(), s, "hello?")
	if err == nil || !strings.Contains(err.Error(), "500") || cl.calls != 5 || len(retries(evs)) != 4 {
		t.Fatalf("err=%v calls=%d retries=%d", err, cl.calls, len(retries(evs)))
	}
	if snap := s.Snapshot(); len(snap) != 1 || snap[0].Text != "hello?" {
		t.Fatalf("user message must stay: %+v", snap)
	}
	// A later turn works once the provider is back.
	cl.steps = append([]func(func(llm.Event)) error{}, okText("back"))
	cl.calls = 0
	if err := r.Continue(context.Background(), s); err != nil {
		t.Fatal(err)
	}
}

func TestCancelDuringBackoffAborts(t *testing.T) {
	cl := &flaky{steps: []func(func(llm.Event)) error{failWith(&llm.HTTPError{Status: 503}), okText("never")}}
	ctx, cancel := context.WithCancel(context.Background())
	var evs []Ev
	vm := jsvm.New(jsvm.Options{Root: "."})
	r := &Runner{D: Deps{Client: cl, VM: vm, Retry: RetryPolicy{Base: time.Hour, Cap: time.Hour}, Emit: func(e Ev) {
		evs = append(evs, e)
		if e.Kind == EvRetry {
			cancel()
		}
	}}}
	start := time.Now()
	err := r.Turn(ctx, NewSession(".", "m"), "hi")
	if !errors.Is(err, context.Canceled) || time.Since(start) > 5*time.Second || cl.calls != 1 {
		t.Fatalf("err=%v calls=%d took %v", err, cl.calls, time.Since(start))
	}
	if evs[len(evs)-1].Kind != EvAborted {
		t.Fatalf("last event %+v", evs[len(evs)-1])
	}
}

func TestBackoffDelays(t *testing.T) {
	p := RetryPolicy{}.norm()
	within := func(got, want time.Duration) {
		t.Helper()
		if got < want*8/10 || got > want*12/10 {
			t.Fatalf("delay %v, want %v ±20%%", got, want)
		}
	}
	within(p.delay(2, 0), time.Second)
	within(p.delay(3, 0), 2*time.Second)
	within(p.delay(4, 0), 4*time.Second)
	within(p.delay(5, 0), 8*time.Second)
	within(p.delay(9, 0), 20*time.Second)
	if p.delay(2, 7*time.Second) != 7*time.Second || p.delay(2, time.Hour) != time.Minute {
		t.Fatal("Retry-After must win and be capped at 60s")
	}
}
