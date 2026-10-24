package cli

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/llm"
)

// gate answers a request only after release is closed, or ends it when the run is stopped.
type gate struct{ release chan struct{} }

func newGate() *gate { return &gate{release: make(chan struct{})} }

func (g *gate) open() { close(g.release) }

func (g *gate) Stream(ctx context.Context, _ llm.Request, emit func(llm.Event)) error {
	select {
	case <-g.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	emit(llm.Event{Kind: llm.EvText, Text: "released"})
	emit(llm.Event{Kind: llm.EvDone})
	return nil
}

func pong(n int) *scripted {
	f := &scripted{}
	for i := 0; i < n; i++ {
		f.steps = append(f.steps, []llm.Event{{Kind: llm.EvText, Text: "pong"}, {Kind: llm.EvDone}})
	}
	return f
}

func waitRunning(t *testing.T, dapi *DaemonAPI, id string, running bool) {
	t.Helper()
	for i := 0; i < 600; i++ {
		if snap, err := dapi.c.Snapshot(context.Background(), id); err == nil && snap.Running == running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("session %s never reached running=%v", id, running)
}

func TestSendForkKeepsHistoryAndRecordsOrigin(t *testing.T) {
	_, _, sess, dapi := start(t, pong(2))
	src := sess.Session.ID
	if res, err := dapi.Send(SendRequest{Text: "first", SessionID: src}); err != nil || !res.Done {
		t.Fatalf("%+v %v", res, err)
	}
	before, _ := dapi.c.Snapshot(context.Background(), src)
	if before.Session.Completed != 0 {
		t.Fatal("CLI send counted toward Done")
	}
	res, err := dapi.Send(SendRequest{Text: "again", Fork: src, Effort: "low", Title: "child"})
	if err != nil || !res.Done || res.ForkOf != src || res.SessionID == src || res.Status != "done" || res.Text != "pong" {
		t.Fatalf("%+v %v", res, err)
	}
	child, err := dapi.SessionShow(res.SessionID)
	if err != nil || child.ForkOf != src || child.SpawnedBy != src || child.Effort != "low" || child.Name != "child" || child.Answer != "pong" {
		t.Fatalf("%+v %v", child, err)
	}
	if child.Turns <= before.TurnCount {
		t.Fatalf("the fork lost history: %d turns, source had %d", child.Turns, before.TurnCount)
	}
	after, _ := dapi.c.Snapshot(context.Background(), src)
	if after.TurnCount != before.TurnCount {
		t.Fatalf("the source changed: %d -> %d turns", before.TurnCount, after.TurnCount)
	}
	if _, err = dapi.Send(SendRequest{Text: "x", Fork: src, Model: "test/nope"}); err == nil {
		t.Fatal("a bad model was accepted")
	}
	items, _ := dapi.Sessions(SessionsRequest{})
	if len(items) != 2 {
		t.Fatalf("a rejected fork left a session behind: %+v", items)
	}
}

func TestSendChangesTheModelOfAnIdleSession(t *testing.T) {
	_, _, sess, dapi := start(t, pong(1))
	src := sess.Session.ID
	res, err := dapi.Send(SendRequest{Text: "hi", SessionID: src, Model: "test/fake", Effort: "low"})
	if err != nil || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "prompt cache") {
		t.Fatalf("%+v %v", res, err)
	}
	if d, _ := dapi.SessionShow(src); d.Effort != "low" || d.Model != "fake" {
		t.Fatalf("%+v", d)
	}
	for _, bad := range []SendRequest{{Model: "test/nope"}, {Effort: "extreme"}, {Model: "elsewhere/fake"}} {
		bad.Text, bad.SessionID = "x", src
		var ae *APIError
		if _, err = dapi.Send(bad); !asAPI(err, &ae) || ae.Code != "bad_request" {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}

func TestSendNewHonorsOptionsAndSessionsFilter(t *testing.T) {
	home, _, sess, dapi := start(t, pong(2))
	dir := config.Paths{Home: home}.SystemPromptsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/scout.md", []byte("You scout."), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := dapi.Send(SendRequest{Text: "go", New: true, Title: "scout 1", System: "scout", Effort: "low", NoAsk: true})
	if err != nil || !res.Done {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err = dapi.Send(SendRequest{Text: "go", New: true, Title: "other"}); err != nil {
		t.Fatal(err)
	}
	mine, err := dapi.Sessions(SessionsRequest{SpawnedBy: "me"})
	if err != nil || len(mine) != 2 {
		t.Fatalf("%+v %v", mine, err)
	}
	for _, s := range mine {
		if s.SpawnedBy != sess.Session.ID {
			t.Fatalf("%+v", s)
		}
	}
	got, _ := dapi.Sessions(SessionsRequest{TitlePrefix: "scout", Status: "idle"})
	if len(got) != 1 || got[0].ID != res.SessionID || got[0].System != "scout" || got[0].Effort != "low" || !got[0].NoAsk || got[0].Turns == 0 {
		t.Fatalf("%+v", got)
	}
	if none, _ := dapi.Sessions(SessionsRequest{Status: "running"}); len(none) != 0 {
		t.Fatalf("%+v", none)
	}
	if none, _ := dapi.Sessions(SessionsRequest{SpawnedBy: "someone-else"}); len(none) != 0 {
		t.Fatalf("%+v", none)
	}
	var ae *APIError
	if _, err = dapi.Send(SendRequest{Text: "x", New: true, System: "missing"}); !asAPI(err, &ae) || ae.Code != "not_found" {
		t.Fatalf("missing system prompt: %v", err)
	}
}

func TestSendWaitsOnAQuestionAndTheAnswerResumes(t *testing.T) {
	g := newGate()
	_, _, sess, dapi := start(t, g)
	id := sess.Session.ID
	asked := make(chan map[string]any, 1)
	type outcome struct {
		res SendResult
		err error
	}
	sent := make(chan outcome, 1)
	go func() {
		r, err := dapi.Send(SendRequest{Text: "go", SessionID: id})
		sent <- outcome{r, err}
	}()
	waitRunning(t, dapi, id, true)
	go func() {
		out, _ := dapi.Ask([]byte(`{"questions":[{"id":"q","text":"Name?","type":"text"}]}`))
		asked <- out
	}()
	var o outcome
	select {
	case o = <-sent:
	case <-time.After(5 * time.Second):
		t.Fatal("send did not return for the question")
	}
	if o.err != nil || o.res.Status != "waiting" || o.res.Question == nil || o.res.Question.Questions[0].ID != "q" {
		t.Fatalf("%+v %v", o.res, o.err)
	}
	d, err := dapi.SessionShow(id)
	if err != nil || d.Status != "needs you" || d.Question == nil || d.Question.ID != o.res.Question.ID {
		t.Fatalf("%+v %v", d, err)
	}
	if err = dapi.SessionAnswer(AnswerRequest{SessionID: id, QuestionID: o.res.Question.ID, Answers: map[string]json.RawMessage{"q": json.RawMessage(`"Ada"`)}}); err != nil {
		t.Fatal(err)
	}
	select {
	case out := <-asked:
		if b, _ := json.Marshal(out); !strings.Contains(string(b), "Ada") {
			t.Fatalf("answers %s", b)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the agent never got the answer")
	}
	g.open()
	waitRunning(t, dapi, id, false)
}

func TestSendTimeoutLeavesTheRunGoingUntilItIsCancelled(t *testing.T) {
	g := newGate()
	_, _, sess, dapi := start(t, g)
	id := sess.Session.ID
	res, err := dapi.Send(SendRequest{Text: "go", SessionID: id, Timeout: 200 * time.Millisecond})
	var ae *APIError
	if !asAPI(err, &ae) || ae.Code != "wait_timeout" || res.Status != "timeout" {
		t.Fatalf("%+v %v", res, err)
	}
	if snap, _ := dapi.c.Snapshot(context.Background(), id); !snap.Running {
		t.Fatal("a timeout must not stop the run")
	}
	if err = dapi.SessionCancel(id); err != nil {
		t.Fatal(err)
	}
	waitRunning(t, dapi, id, false)
	if err = dapi.SessionCancel(id); err == nil {
		t.Fatal("cancelling an idle session must be an error")
	}
}

func TestSendInterruptCancelsOnlyTheRunItCreated(t *testing.T) {
	for _, tc := range []struct {
		name      string
		req       SendRequest
		cancelled bool
	}{
		{"created", SendRequest{New: true}, true},
		{"created and kept", SendRequest{New: true, KeepRunning: true}, false},
		{"existing", SendRequest{}, false},
		{"existing with cancel-on-exit", SendRequest{CancelOnExit: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newGate()
			_, _, sess, dapi := start(t, g)
			req := tc.req
			req.Text = "go"
			if !req.New {
				req.SessionID = sess.Session.ID
			}
			done := make(chan SendResult, 1)
			go func() { r, _ := dapi.Send(req); done <- r }()
			var id string
			for i := 0; i < 600 && id == ""; i++ {
				items, _ := dapi.c.ListSessions(context.Background(), api.SessionsListRequest{})
				for _, s := range items.Sessions {
					if s.Running {
						id = s.ID
					}
				}
				time.Sleep(5 * time.Millisecond)
			}
			if id == "" {
				t.Fatal("the run never started")
			}
			time.Sleep(50 * time.Millisecond) // the signal handler of Send is installed before the run starts
			_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
			var res SendResult
			select {
			case res = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("send ignored the interrupt")
			}
			if tc.cancelled {
				if res.Status != "cancelled" {
					t.Fatalf("%+v", res)
				}
				waitRunning(t, dapi, id, false)
				return
			}
			if res.Status != "interrupted" {
				t.Fatalf("%+v", res)
			}
			if snap, _ := dapi.c.Snapshot(context.Background(), id); !snap.Running {
				t.Fatal("the run was cancelled")
			}
			g.open()
		})
	}
}

func TestTailStreamsAndFilters(t *testing.T) {
	g := newGate()
	_, _, sess, dapi := start(t, g)
	id := sess.Session.ID
	go func() { _, _ = dapi.Send(SendRequest{Text: "go", SessionID: id}) }()
	waitRunning(t, dapi, id, true)
	var mu sync.Mutex
	var events []Event
	finished := make(chan error, 1)
	go func() {
		finished <- dapi.Tail(TailRequest{IDs: []string{id}, Events: []string{"text_delta", "done"}}, func(e Event) error {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
			return nil
		})
	}()
	time.Sleep(100 * time.Millisecond)
	g.open()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tail did not end with the run")
	}
	mu.Lock()
	defer mu.Unlock()
	var types []string
	for _, e := range events {
		types = append(types, e.Type)
		if e.Session != id || e.Boot == "" || e.Sequence == 0 {
			t.Fatalf("event without origin: %+v", e)
		}
	}
	if strings.Join(types, ",") != "text_delta,done" {
		t.Fatalf("filtered events: %v", types)
	}
	// The run is over: the session is idle and gives its turns; --from skips some.
	var turns []Event
	err := dapi.Tail(TailRequest{IDs: []string{id}}, func(e Event) error { turns = append(turns, e); return nil })
	if err != nil || len(turns) < 2 || turns[0].Type != "turn" {
		t.Fatalf("%+v %v", turns, err)
	}
	var rest []Event
	_ = dapi.Tail(TailRequest{IDs: []string{id}, From: 1, HasFrom: true}, func(e Event) error { rest = append(rest, e); return nil })
	if len(rest) != len(turns)-1 || rest[0].Sequence != 2 {
		t.Fatalf("--from 1: %+v", rest)
	}
}

func TestTailChildrenOfFollowsWorkersUntilTheyEnd(t *testing.T) {
	g := newGate()
	_, _, _, dapi := start(t, g)
	var ids []string
	for _, title := range []string{"w1", "w2"} {
		res, err := dapi.Send(SendRequest{Text: "go", New: true, Detach: true, Title: title})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, res.SessionID)
	}
	var mu sync.Mutex
	seen := map[string]bool{}
	finished := make(chan error, 1)
	go func() {
		finished <- dapi.Tail(TailRequest{ChildrenOf: "me", Events: []string{"done"}}, func(e Event) error {
			mu.Lock()
			seen[e.Session] = true
			mu.Unlock()
			return nil
		})
	}()
	time.Sleep(300 * time.Millisecond)
	g.open()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("tail --children-of did not end")
	}
	mu.Lock()
	defer mu.Unlock()
	if !seen[ids[0]] || !seen[ids[1]] {
		t.Fatalf("done events seen for %v, want %v", seen, ids)
	}
}

func TestWorkerLimitIsForbiddenAndLeavesNothingBehind(t *testing.T) {
	g := newGate()
	_, _, _, dapi := start(t, g)
	defer g.open()
	if _, err := dapi.c.PatchSettings(context.Background(), map[string]any{"max_workers": 1}, nil); err != nil {
		t.Fatal(err)
	}
	first, err := dapi.Send(SendRequest{Text: "go", New: true, Detach: true})
	if err != nil {
		t.Fatal(err)
	}
	waitRunning(t, dapi, first.SessionID, true)
	var ae *APIError
	_, err = dapi.Send(SendRequest{Text: "go", New: true, Detach: true})
	if !asAPI(err, &ae) || ae.Code != api.CodeForbidden {
		t.Fatalf("second worker: %v", err)
	}
	code, _, errs := runE2E(t, dapi, "send", "--new", "--detach", "x")
	if code != ExitForbidden || !strings.Contains(errs, "max_workers") {
		t.Fatalf("exit %d %q", code, errs)
	}
}
