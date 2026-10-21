package engine

import (
	"context"
	"fmt"
	"meno/internal/agent"
	"meno/internal/client"
	"meno/internal/daemon"
	"meno/internal/llm"
	"meno/internal/proto"
	"strings"
	"testing"
)

func TestStreamingFragmentsDoNotWriteOrFanOut(t *testing.T) {
	e, hub := fixture(t, &fakeProvider{})
	id := usedSession(t, e)
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.store.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Exec(`CREATE TABLE write_audit(n INTEGER); INSERT INTO write_audit VALUES(0)`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"sessions", "turns", "requests"} {
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			q := fmt.Sprintf("CREATE TRIGGER audit_%s_%s AFTER %s ON %s BEGIN UPDATE write_audit SET n=n+1; END", table, op, op, table)
			if _, err = d.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
	}
	global := e.global.(*daemon.GlobalHub)
	before := global.Sequence()
	cursor := hub.Sequence(id)
	for i := 0; i < 200; i++ {
		e.agentEvent(s, agent.Ev{Kind: agent.EvText, Text: "x"})
		e.agentEvent(s, agent.Ev{Kind: agent.EvThink, Thinking: "y"})
		e.agentEvent(s, agent.Ev{Kind: agent.EvCode, CallID: "c", Code: strings.Repeat("z", i+1)})
	}
	var n int
	if err = d.QueryRow("SELECT n FROM write_audit").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 || global.Sequence() != before {
		t.Fatalf("writes=%d global events=%d", n, global.Sequence()-before)
	}
	sn, err := e.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(sn.Live.Text) != 200 || len(sn.Live.Thinking) != 200 || len(sn.Live.Calls) != 1 || hub.Sequence(id)-cursor != 600 {
		t.Fatalf("lost fragments: text=%d thinking=%d calls=%d events=%d", len(sn.Live.Text), len(sn.Live.Thinking), len(sn.Live.Calls), hub.Sequence(id)-cursor)
	}
	for _, kind := range []string{agent.EvRequest, agent.EvRequestRecord, agent.EvInjected, agent.EvJSEnd, agent.EvRetry, agent.EvError, agent.EvAborted, agent.EvDone} {
		t.Run(kind, func(t *testing.T) {
			old := n
			e.agentEvent(s, agent.Ev{Kind: kind, CallID: "c"})
			if err = d.QueryRow("SELECT n FROM write_audit").Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n <= old {
				t.Fatalf("%s failed to persist boundary", kind)
			}
		})
	}
}

func TestNextRequestPersistsBatchTurnsAfterJSEnd(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := usedSession(t, e)
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	e.agentEvent(s, agent.Ev{Kind: agent.EvJSEnd, CallID: "batch"})
	s.session.Append(agent.Turn{Role: "assistant", CallID: "batch", Code: "echo ok", Output: "ok"})
	e.agentEvent(s, agent.Ev{Kind: agent.EvRequest})
	d, err := e.store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = d.QueryRow("SELECT count(*) FROM turns WHERE session_id=? AND call_id='batch'", id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("tool turn not durable at next request: %d %v", count, err)
	}
}

func TestSessionChangePayloadCarriesEmptyValuesAndAttention(t *testing.T) {
	e, hub := fixture(t, &fakeProvider{})
	id := usedSession(t, e)
	seq := hub.Sequence(id)
	events, stop, _ := hub.Subscribe(id, seq, "boot")
	defer stop()
	effort, provider := "", "other"
	if _, err := e.PatchSession(context.Background(), id, proto.PatchSessionRequest{Provider: &provider, Effort: &effort}); err != nil {
		t.Fatal(err)
	}
	ev := <-events
	var d proto.ChangedData
	if err := decodeData(ev.Data, &d); err != nil || d.Metadata == nil {
		t.Fatalf("missing metadata %s %v", ev.Data, err)
	}
	if d.Metadata.Provider != "other" || d.Metadata.Effort != "" || d.Metadata.Session.ID != id {
		t.Fatalf("metadata %+v", d.Metadata)
	}
}

func TestCompletionPublishesFinalStatsAndTerminalOnce(t *testing.T) {
	e, hub := fixture(t, &fakeProvider{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "answer"}, {Kind: llm.EvDone}}}})
	id := newSession(t, e)
	if _, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "start"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	final, err := e.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	events, stop, _ := hub.Subscribe(id, 0, "boot")
	defer stop()
	terminals, updates := 0, 0
	var mirror client.Mirror
	mirror.Load(proto.Snapshot{Session: proto.SessionItem{ID: id}, Version: proto.SessionVersion{BootID: "boot"}})
	for len(events) > 0 {
		ev := <-events
		mirror.Apply(ev)
		if ev.Type == proto.EventDone {
			terminals++
		}
		if ev.Type == proto.EventTurnUpdated {
			updates++
		}
	}
	state := mirror.Snapshot()
	if terminals != 1 || updates != 1 || state.Running || state.Session.Completed != final.Session.Completed {
		t.Fatalf("terminal=%d updates=%d projection=%+v", terminals, updates, state)
	}
	last := state.Turns[len(state.Turns)-1]
	if last.Run == nil || last.Run.Turns != 1 {
		t.Fatalf("final stats %+v", last.Run)
	}
}
