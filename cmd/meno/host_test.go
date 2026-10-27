package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/db"
	"meno/internal/engine"
	"meno/internal/llm"
	"meno/internal/stdio"
)

// scripted answers each request with the next list of events.
type scripted struct{ steps [][]llm.Event }

func (f *scripted) Stream(_ context.Context, _ llm.Request, emit func(llm.Event)) error {
	step := []llm.Event{{Kind: llm.EvText, Text: "done"}, {Kind: llm.EvDone}}
	if len(f.steps) > 0 {
		step, f.steps = f.steps[0], f.steps[1:]
	}
	for _, e := range step {
		emit(e)
	}
	return nil
}

func text(s string) []llm.Event { return []llm.Event{{Kind: llm.EvText, Text: s}, {Kind: llm.EvDone}} }

// testHost starts a host with a fake model in a fresh MENO_HOME.
func testHost(t *testing.T, f llm.Client) (*host, string) {
	t.Helper()
	home := t.TempDir()
	st := config.NewStore(config.Paths{Home: home})
	s := config.DefaultSettings()
	s.Provider, s.Model = "test", "fake"
	if err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	p := llm.Provider{Name: "test", BaseURL: "http://127.0.0.1", Protocol: llm.ProtoResponses, Models: map[string]llm.ModelSettings{"fake": {Context: 100000}}}
	if err := st.SaveProviders([]llm.Provider{p}); err != nil {
		t.Fatal(err)
	}
	h, err := startHost(home, func(o *engine.Options) { o.ProviderClient = func(llm.Provider) llm.Client { return f } })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close(); db.CloseAll() })
	return h, home
}

func TestHeadlessPrintsTheAnswerAndSavesTheSession(t *testing.T) {
	h, _ := testHost(t, &scripted{steps: [][]llm.Event{text("pong")}})
	res, err := headlessRun(context.Background(), h.client, launch{print: true}, "ping", io.Discard)
	if err != nil || res.Status != "done" || strings.TrimSpace(res.Text) != "pong" {
		t.Fatalf("%+v %v", res, err)
	}
	list, err := h.client.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil || len(list.Sessions) != 1 || list.Sessions[0].ID != res.SessionID {
		t.Fatalf("a headless session is stored: %+v %v", list, err)
	}
}

func TestHeadlessNoSaveLeavesNothing(t *testing.T) {
	h, _ := testHost(t, &scripted{steps: [][]llm.Event{text("pong")}})
	if _, err := headlessRun(context.Background(), h.client, launch{print: true, noSave: true}, "ping", io.Discard); err != nil {
		t.Fatal(err)
	}
	list, _ := h.client.ListSessions(context.Background(), api.SessionsListRequest{})
	if len(list.Sessions) != 0 {
		t.Fatalf("--no-save kept %+v", list.Sessions)
	}
}

func TestHeadlessResumeAndContinueUseTheStoredSession(t *testing.T) {
	h, _ := testHost(t, &scripted{steps: [][]llm.Event{text("one"), text("two"), text("three")}})
	first, err := headlessRun(context.Background(), h.client, launch{print: true}, "first", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	second, err := headlessRun(context.Background(), h.client, launch{print: true, resume: first.SessionID}, "second", io.Discard)
	if err != nil || second.SessionID != first.SessionID || strings.TrimSpace(second.Text) != "two" {
		t.Fatalf("resume %+v %v", second, err)
	}
	third, err := headlessRun(context.Background(), h.client, launch{print: true, cont: true, cwd: cwd}, "third", io.Discard)
	if err != nil || third.SessionID != first.SessionID {
		t.Fatalf("continue %+v %v", third, err)
	}
	snap, _ := h.client.Snapshot(context.Background(), first.SessionID)
	if snap.TurnCount < 6 {
		t.Fatalf("the session has %d turns, want the three exchanges", snap.TurnCount)
	}
}

func TestContinueWithoutASessionFails(t *testing.T) {
	h, _ := testHost(t, &scripted{})
	_, err := headlessRun(context.Background(), h.client, launch{print: true, cont: true, cwd: t.TempDir()}, "x", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no session to continue") {
		t.Fatalf("%v", err)
	}
}

func TestStdioRunsASessionAndStreamsEvents(t *testing.T) {
	h, _ := testHost(t, &scripted{steps: [][]llm.Event{text("hello over rpc")}})
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- stdio.Run(context.Background(), inR, outW, h.client, stdio.Options{}); outW.Close() }()
	dec := json.NewDecoder(outR)
	send := func(id int, method string, params any) {
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		_, _ = inW.Write(append(b, '\n'))
	}
	type msg struct {
		ID     *int            `json:"id"`
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
		Params json.RawMessage `json:"params"`
	}
	next := func() msg {
		var m msg
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	reply := func(id int) msg {
		for {
			if m := next(); m.ID != nil && *m.ID == id {
				return m
			}
		}
	}
	cwd, _ := os.Getwd()
	send(1, "session.create", map[string]any{"cwd": cwd})
	var created api.SessionCreateResponse
	if m := reply(1); m.Error != nil || json.Unmarshal(m.Result, &created) != nil || created.Session.ID == "" {
		t.Fatalf("create: %+v", m)
	}
	id := created.Session.ID
	send(2, "subscribe", map[string]any{"session": id})
	reply(2)
	send(3, "session.message", map[string]any{"id": id, "text": "hi"})
	if m := reply(3); m.Error != nil {
		t.Fatalf("message: %+v", m.Error)
	}
	deadline := time.After(10 * time.Second)
	finished := make(chan bool, 1)
	go func() {
		for {
			m := next()
			if m.Method != "event" {
				continue
			}
			var p struct {
				Event api.Event `json:"event"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if p.Event.Type == api.EventDone {
				finished <- true
				return
			}
		}
	}()
	select {
	case <-finished:
	case <-deadline:
		t.Fatal("no done event")
	}
	send(4, "session.get", map[string]any{"id": "missing"})
	if m := reply(4); m.Error == nil {
		t.Fatal("an unknown session must be an error")
	}
	inW.Close() // end of input stops the server
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
