package engine

import (
	"context"
	"encoding/json"
	"errors"
	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/proto"
	"sync"
	"testing"
	"time"
)

type fakeProvider struct {
	mu    sync.Mutex
	steps [][]llm.Event
	calls int
	wait  bool
	seen  chan struct{}
}

func (f *fakeProvider) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	f.mu.Lock()
	f.calls++
	n := f.calls
	wait := f.wait
	f.mu.Unlock()
	if wait {
		if f.seen != nil {
			select {
			case f.seen <- struct{}{}:
			default:
			}
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if n > len(f.steps) {
		return errors.New("unexpected provider request")
	}
	for _, ev := range f.steps[n-1] {
		emit(ev)
	}
	return nil
}
func fixture(t *testing.T, f *fakeProvider) (*Engine, *daemon.Hub) {
	t.Helper()
	home := t.TempDir()
	store := config.NewStore(config.Paths{Home: home})
	st := config.DefaultSettings()
	st.Provider = "test"
	st.Model = "fake"
	if err := store.SaveSettings(st); err != nil {
		t.Fatal(err)
	}
	p := llm.Provider{Name: "test", BaseURL: "http://127.0.0.1", Protocol: llm.ProtoResponses, Models: map[string]llm.ModelSettings{"fake": {Context: 100000}}}
	if err := store.SaveProviders([]llm.Provider{p}); err != nil {
		t.Fatal(err)
	}
	hub := daemon.NewHub("boot", 2000)
	global := daemon.NewGlobalHub("boot", 1000)
	e, err := New(Options{MCP: mcp.NewManager(), Home: home, EventHub: hub, GlobalHub: global, ProviderClient: func(llm.Provider) llm.Client { return f }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close(); db.CloseAll() })
	return e, hub
}
func newSession(t *testing.T, e *Engine) string {
	t.Helper()
	r, err := e.CreateSession(context.Background(), proto.SessionCreateRequest{Cwd: t.TempDir(), ClientID: "create"})
	if err != nil {
		t.Fatal(err)
	}
	return r.Session.ID
}
func waitFor(t *testing.T, e *Engine, id string, pred func(*runState) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s, err := e.load(context.Background(), id)
		if err == nil {
			s.mu.Lock()
			ok := pred(s)
			s.mu.Unlock()
			if ok {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("session state timeout")
}
func TestRunSurvivesClientCloseAndPublishesEvents(t *testing.T) {
	f := &fakeProvider{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "hello"}, {Kind: llm.EvDone}}}}
	e, h := fixture(t, f)
	id := newSession(t, e)
	ctx, cancel := context.WithCancel(context.Background())
	r, err := e.Message(ctx, id, proto.MessageRequest{Text: "hi", ClientID: "msg"})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	snap, err := e.Snapshot(context.Background(), id)
	if err != nil || len(snap.Turns) < 2 {
		t.Fatalf("snapshot %+v %v", snap, err)
	}
	ch, stop, resync := h.Subscribe(id, 0, "boot")
	defer stop()
	if resync {
		t.Fatal("resync")
	}
	text, done := false, false
	for len(ch) > 0 {
		ev := <-ch
		text = text || ev.Type == proto.EventTextDelta
		done = done || ev.Type == proto.EventDone
	}
	if !text || !done {
		t.Fatalf("missing events text=%v done=%v request=%s", text, done, r.RequestID)
	}
}
func TestTwoClientsSteerAndStop(t *testing.T) {
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	r, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "start", ClientID: "one"})
	if err != nil {
		t.Fatal(err)
	}
	<-f.seen
	q, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "steer", ClientID: "two"})
	if err != nil || !q.Queued || q.RequestID != r.RequestID {
		t.Fatalf("queue %+v %v", q, err)
	}
	if err = e.Stop(context.Background(), id, proto.StopRequest{RequestID: r.RequestID}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
}
func TestPOSTClientIDRetryIsSafe(t *testing.T) {
	f := &fakeProvider{}
	e, _ := fixture(t, f)
	calls := 0
	fn := func() (any, error) {
		calls++
		return e.CreateSession(context.Background(), proto.SessionCreateRequest{Cwd: t.TempDir(), ClientID: "id"})
	}
	a, err := e.ApplyClientID(context.Background(), "id", fn)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.ApplyClientID(context.Background(), "id", fn)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || a.(proto.SessionCreateResponse).Session.ID != b.(proto.SessionCreateResponse).Session.ID {
		t.Fatalf("calls=%d", calls)
	}
}
func TestAskPausesThenResumes(t *testing.T) {
	f := &fakeProvider{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "answered"}, {Kind: llm.EvDone}}}}
	e, h := fixture(t, f)
	id := newSession(t, e)
	snap0, _ := e.Snapshot(context.Background(), id)
	// The agent's shell command "meno ask" posts the form and blocks.
	done := make(chan error, 1)
	go func() {
		_, err := e.Resource(context.Background(), "POST", "/v1/sessions/"+id+"/ask", "", proto.ResourceRequest{
			Token: snap0.AgentToken, Data: json.RawMessage(`{"form":{"questions":[{"id":"choice","text":"Pick","type":"single","options":[{"id":"x","label":"X"}]}]}}`)})
		done <- err
	}()
	waitFor(t, e, id, func(s *runState) bool { return s.ask != nil })
	snap, _ := e.Snapshot(context.Background(), id)
	if snap.Ask == nil {
		t.Fatal("ask missing")
	}
	if err := e.AnswerAsk(context.Background(), id, snap.Ask.AskID, proto.AskAnswerRequest{Answers: map[string]json.RawMessage{"choice": json.RawMessage("\"x\"")}}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return s.ask == nil })
	ch, stop, _ := h.Subscribe(id, 0, "boot")
	defer stop()
	opened, closed := false, false
	for len(ch) > 0 {
		ev := <-ch
		opened = opened || ev.Type == proto.EventAskOpen
		closed = closed || ev.Type == proto.EventAskClosed
	}
	if !opened || !closed {
		t.Fatalf("ask events open=%v close=%v", opened, closed)
	}
}
func TestCrashRecoveryMarksTasksInterrupted(t *testing.T) {
	home := t.TempDir()
	store := config.NewStore(config.Paths{Home: home})
	if err := store.SaveSettings(config.DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	sess := agent.NewSession(t.TempDir(), "m")
	sess.ID = "unfinished"
	sess.Append(agent.Turn{Role: llm.RoleUser, Text: "running"})
	if err = agent.SaveSession(home, sess); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Exec("INSERT INTO tasks(id,session_id,owner,name,command,cwd,status,notify,timeout_ms,started_at) VALUES('t1',?,'','','sleep','/tmp','running',0,0,'')", sess.ID); err != nil {
		t.Fatal(err)
	}
	e, err := New(Options{MCP: mcp.NewManager(), Home: home})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	var status string
	if err = d.QueryRow("SELECT status FROM tasks WHERE id='t1'").Scan(&status); err != nil || status != "interrupted" {
		t.Fatalf("task status=%q err=%v", status, err)
	}
}

func TestTokenIsEphemeral(t *testing.T) {
	f := &fakeProvider{}
	e, _ := fixture(t, f)
	id := usedSession(t, e)
	snap, err := e.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	token := snap.AgentToken
	if token == "" {
		t.Fatal("missing agent token")
	}
	path := "/v1/sessions/" + id + "/tasks"
	if _, err = e.Resource(context.Background(), "GET", path, "", proto.ResourceRequest{Token: token}); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	s := e.sessions[id]
	e.mu.Unlock()
	s.mu.Lock()
	err = e.save(s)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := agent.LoadSession(e.home, id)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != id {
		t.Fatal("session missing")
	}
}
