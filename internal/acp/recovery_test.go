package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/llm"
)

func TestListParamsAndCursor(t *testing.T) {
	c, _ := daemonRig(t, &gated{})
	p := startPeer(t, c)
	cwd := t.TempDir()
	id := result(t, p.call("session/new", map[string]any{"cwd": cwd}))["sessionId"].(string)
	promptText(t, p, id, "persist")
	r := result(t, p.call("session/list", map[string]any{"cwd": cwd}))
	list := r["sessions"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["sessionId"] != id {
		t.Fatal(r)
	}
	for _, params := range []any{[]any{}, map[string]any{"cwd": "relative"}, map[string]any{"cursor": "invalid"}} {
		if m := p.call("session/list", params); m.Error == nil {
			t.Fatalf("invalid list params accepted: %v", params)
		}
	}
	for _, params := range []any{map[string]any{"sessionId": "missing"}, map[string]any{"sessionId": id, "cwd": t.TempDir()}} {
		if m := p.call("session/load", params); m.Error == nil {
			t.Fatal("invalid load accepted")
		}
	}
}

func TestReplayResetSendsMissedTurnsOnce(t *testing.T) {
	b := &scriptBackend{up: make(chan struct{})}
	p := startPeer(t, b)
	newSessionOn(t, p)
	<-b.up
	b.push(client.ProjectionUpdate{Reset: true, Changed: client.AllChanged, Values: client.MirrorState{Turns: []api.Turn{stepTurn("missed", "recovered")}}})
	b.push(client.ProjectionUpdate{Reset: true, Changed: client.AllChanged, Values: client.MirrorState{Turns: []api.Turn{stepTurn("missed", "recovered")}}})
	p.waitFor("reset replay", func() bool { return p.text("agent_message_chunk") == "recovered" })
}

type failedProvider struct{}

func (failedProvider) Stream(context.Context, llm.Request, func(llm.Event)) error {
	return &llm.HTTPError{Status: 400, Body: "private-provider-secret"}
}
func TestProviderErrorIsNotSuccessfulOrExposed(t *testing.T) {
	c, _ := daemonRig(t, failedProvider{})
	p := startPeer(t, c)
	id := newSessionOn(t, p)
	m := p.call("session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]string{"type": "text", "text": "hello"}}})
	if m.Error == nil || strings.Contains(string(mustJSON(m)), "private-provider-secret") {
		t.Fatalf("bad provider outcome: %s", mustJSON(m))
	}
}

func TestThoughtOnlyAndTwoSessionCancel(t *testing.T) {
	g := &gated{hold: make(chan struct{}), started: make(chan struct{}, 2), steps: [][]llm.Event{{{Kind: llm.EvThinking, Text: "thought-only"}, {Kind: llm.EvDone}}}}
	c, _ := daemonRig(t, g)
	p := startPeer(t, c)
	a := newSessionOn(t, p)
	b := newSessionOn(t, p)
	params := func(id string) any {
		return map[string]any{"sessionId": id, "prompt": []any{map[string]string{"type": "text", "text": "go"}}}
	}
	ac := p.start("session/prompt", params(a))
	<-g.started
	bc := p.start("session/prompt", params(b))
	<-g.started
	if m := p.call("session/prompt", params(a)); m.Error == nil {
		t.Fatal("concurrent prompt accepted")
	}
	p.notify("session/cancel", map[string]string{"sessionId": a})
	p.notify("session/cancel", map[string]string{"sessionId": a})
	if result(t, <-ac)["stopReason"] != "cancelled" {
		t.Fatal("cancel failed")
	}
	select {
	case <-bc:
		t.Fatal("cancel affected other session")
	case <-time.After(30 * time.Millisecond):
	}
	close(g.hold)
	if result(t, <-bc)["stopReason"] != "end_turn" {
		t.Fatal("other session did not finish")
	}
	if p.text("agent_thought_chunk") != "thought-only" || strings.TrimSpace(strings.ReplaceAll(p.text("agent_message_chunk"), "[aborted]", "")) != "" {
		t.Fatalf("thought/message conflated: thought=%q text=%q", p.text("agent_thought_chunk"), p.text("agent_message_chunk"))
	}
}

func TestListRejectsMalformedJSON(t *testing.T) {
	s := &Server{}
	if _, err := s.listSessions(json.RawMessage(`{"cwd":`)); err == nil {
		t.Fatal("malformed params accepted")
	}
}

func TestLoadOpenQuestionWithEmptyMCP(t *testing.T) {
	// Existing question/EOF integration covers reattachment; this explicitly
	// asserts that an empty MCP list does not replace or stop an active run.
	g := &gated{hold: make(chan struct{}), started: make(chan struct{}, 1)}
	c, _ := daemonRig(t, g)
	p := startPeer(t, c)
	id := newSessionOn(t, p)
	ch := p.start("session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]string{"type": "text", "text": "go"}}})
	<-g.started
	q := startPeer(t, c)
	if m := q.call("session/load", map[string]any{"sessionId": id, "mcpServers": []any{}}); m.Error != nil {
		t.Fatal(m.Error)
	}
	p.notify("session/cancel", map[string]string{"sessionId": id})
	result(t, <-ch)
}

func TestConnectionContextCancellationStopsRun(t *testing.T) {
	b := &slowBackend{release: make(chan struct{}), entered: make(chan struct{})}
	inR, inW := io.Pipe()
	defer inW.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, inR, io.Discard, b, Options{}) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("context cancellation did not close the connection")
	}
}

type listBackend struct {
	Backend
	items []api.SessionItem
}

func (b *listBackend) ListSessions(context.Context, api.SessionsListRequest) (api.SessionsListResponse, error) {
	return api.SessionsListResponse{Sessions: b.items}, nil
}
func TestListPaginationAndStableOrder(t *testing.T) {
	b := &listBackend{}
	for i := 204; i >= 0; i-- {
		b.items = append(b.items, api.SessionItem{ID: fmt.Sprintf("s%03d", i), Cwd: "/tmp", UpdatedAt: "2026-10-24T00:00:00Z"})
	}
	s := &Server{backend: b, ctx: context.Background()}
	page, err := s.listSessions(json.RawMessage(`{"cwd":"/tmp"}`))
	if err != nil {
		t.Fatal(err)
	}
	first := page.(map[string]any)
	items := first["sessions"].([]map[string]any)
	if len(items) != 200 || items[0]["sessionId"] != "s000" {
		t.Fatal("unstable page")
	}
	raw := mustJSON(map[string]any{"cwd": "/tmp", "cursor": first["nextCursor"]})
	page, err = s.listSessions(raw)
	if err != nil {
		t.Fatal(err)
	}
	last := page.(map[string]any)
	if len(last["sessions"].([]map[string]any)) != 5 || last["nextCursor"] != nil {
		t.Fatal("bad final page")
	}
	if _, err = s.listSessions(mustJSON(map[string]any{"cwd": "/other", "cursor": first["nextCursor"]})); err == nil {
		t.Fatal("cursor filter mismatch accepted")
	}
}

func TestLongHistoryLoadOnce(t *testing.T) {
	c, _ := daemonRig(t, &gated{})
	p := startPeer(t, c)
	id := newSessionOn(t, p)
	for i := 0; i < 12; i++ {
		promptText(t, p, id, fmt.Sprintf("message-%02d", i))
	}
	q := startPeer(t, c)
	result(t, q.call("session/load", map[string]any{"sessionId": id}))
	for i := 0; i < 12; i++ {
		if strings.Count(q.text("user_message_chunk"), fmt.Sprintf("message-%02d", i)) != 1 {
			t.Fatal("history missing or duplicated")
		}
	}
	if strings.Count(q.text("agent_message_chunk"), "ok") != 12 {
		t.Fatal("assistant history missing/duplicated")
	}
}
