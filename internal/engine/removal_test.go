package engine

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"meno/internal/agent"
	"meno/internal/db"
	"meno/internal/proto"
)

func TestQueueEventsCarryRemainingMessages(t *testing.T) {
	e, hub := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.running = true
	s.requestID = "request"
	s.mu.Unlock()
	ch, stop, _ := hub.Subscribe(id, hub.Sequence(id), "boot")
	defer stop()
	_, err = e.Message(context.Background(), id, proto.MessageRequest{Text: "steer"})
	if err != nil {
		t.Fatal(err)
	}
	s.queue.Take()
	e.agentEvent(s, agent.Ev{Kind: agent.EvInjected})
	var got [][]string
	for len(ch) > 0 {
		ev := <-ch
		if ev.Type != proto.EventInjected {
			continue
		}
		var data proto.InjectedData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			t.Fatal(err)
		}
		if data.Queued == nil {
			t.Fatal("queue must be present even when empty")
		}
		got = append(got, data.Queued)
	}
	if !reflect.DeepEqual(got, [][]string{{"steer"}, {}}) {
		t.Fatalf("queue events = %#v", got)
	}
}

func TestRemovedAgentFeaturesAreUnavailableAndHistoryIsUnchanged(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	ctx := context.Background()
	if _, err := e.PatchSession(ctx, id, proto.PatchSessionRequest{Modes: &[]string{"subagents"}}); err == nil {
		t.Fatal("removed mode must be rejected")
	}
	s, _ := e.load(ctx, id)
	if _, err := e.Resource(ctx, "POST", "/v1/sessions/"+id+"/agents/spawn", "", proto.ResourceRequest{Token: s.token.value}); err == nil {
		t.Fatal("removed resource must be rejected")
	}
	child := agent.NewSession(t.TempDir(), "fake")
	child.Kind, child.ParentID = agent.KindChild, id
	if err := agent.SaveSession(e.home, child); err != nil {
		t.Fatal(err)
	}
	d, err := db.Open(e.home)
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Exec("INSERT INTO agents(id,parent_id,task,status,created_at) VALUES(?,?,'old task','running','')", child.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := New(Options{Home: e.home})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var status string
	if err := d.QueryRow("SELECT status FROM agents WHERE id=?", child.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "running" {
		t.Fatalf("historical status changed: %q", status)
	}
}
