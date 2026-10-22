package acp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/llm"
)

func TestEOFDuringExecutionStopsTheRunButNotTheDaemon(t *testing.T) {
	g := &gated{hold: make(chan struct{}), started: make(chan struct{}, 1)}
	c, _ := daemonRig(t, g)
	p := startPeer(t, c)
	id := newSessionOn(t, p)
	p.start("session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text", "text": "work"}}})
	<-g.started
	_ = p.in.Close()
	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return at EOF")
	}
	p.t.Cleanup(func() {})
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap, err := c.Snapshot(context.Background(), id)
		if err != nil {
			t.Fatalf("daemon went away: %v", err)
		}
		if !snap.Running {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("run is still executing after EOF")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestEOFDuringQuestionKeepsFormAndRun(t *testing.T) {
	g := &gated{hold: make(chan struct{}), started: make(chan struct{}, 1), steps: [][]llm.Event{{{Kind: llm.EvText, Text: "resumed"}, {Kind: llm.EvDone}}}}
	c, eng := daemonRig(t, g)
	p := startPeer(t, c)
	id := newSessionOn(t, p)
	snap, _ := c.Snapshot(context.Background(), id)
	ch := p.start("session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text", "text": "ask"}}})
	<-g.started
	go func() {
		_, _ = eng.Resource(context.Background(), "POST", "/v1/sessions/"+id+"/ask", "", api.ResourceRequest{Token: snap.AgentToken,
			Data: json.RawMessage(`{"form":{"questions":[{"id":"c","text":"Pick","type":"text"}]}}`)})
	}()
	result(t, <-ch)
	_ = p.in.Close()
	<-p.done
	time.Sleep(200 * time.Millisecond)
	after, err := c.Snapshot(context.Background(), id)
	if err != nil || after.Ask == nil || !after.Running {
		t.Fatalf("question or run lost at EOF: ask=%v running=%v err=%v", after.Ask, after.Running, err)
	}
	// A new connection finds the open form and finishes the same run.
	q := startPeer(t, c)
	q.call("initialize", map[string]any{"protocolVersion": 1})
	q.call("session/load", map[string]any{"sessionId": id, "cwd": "", "mcpServers": []any{}})
	q.waitFor("question re-rendered", func() bool { return len(q.text("agent_message_chunk")) > 0 })
	close(g.hold)
	r := promptText(t, q, id, `/answer `+after.Ask.AskID+` {"c":"typed text"}`)
	if r["stopReason"] != "end_turn" {
		t.Fatalf("answer: %v", r)
	}
}
