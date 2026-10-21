package engine

import (
	"context"
	"encoding/json"
	"meno/internal/agent"
	"meno/internal/proto"
	"testing"
)

func TestDurableReadAcknowledgmentDoesNotClearNewerRun(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := usedSession(t, e)
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = agent.SaveCompletion(e.home, s.session); err != nil {
			t.Fatal(err)
		}
	}
	raw := json.RawMessage(`{"through":1}`)
	if _, err = e.readCompletion(context.Background(), id, raw); err != nil {
		t.Fatal(err)
	}
	completed, read := e.attention(id)
	if completed != 2 || read != 1 {
		t.Fatalf("%d/%d", completed, read)
	}
	if _, err = e.readCompletion(context.Background(), id, json.RawMessage(`{"through":0}`)); err != nil {
		t.Fatal(err)
	}
	_, read = e.attention(id)
	if read != 1 {
		t.Fatal("watermark moved backward")
	}
	list, err := e.ListSessions(context.Background(), proto.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if list.Sessions[0].Completed != 2 || list.Sessions[0].ReadThrough != 1 {
		t.Fatal("list lost unread state")
	}
	if _, err = e.readCompletion(context.Background(), id, json.RawMessage(`{"through":2}`)); err != nil {
		t.Fatal(err)
	}
	snap, err := e.Snapshot(context.Background(), id)
	if err != nil || snap.Session.ReadThrough != 2 {
		t.Fatal("snapshot lost read state", err)
	}
}

func TestRemovedFeatureResourcesFail(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	for _, path := range []string{"modes", "goal/done"} {
		if _, err := e.Resource(context.Background(), "POST", "/v1/sessions/"+id+"/"+path, "", proto.ResourceRequest{Data: json.RawMessage(`{}`)}); err == nil {
			t.Fatal("removed resource accepted", path)
		}
	}
}

func TestGlobalStateCountsPendingAskSeparately(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := usedSession(t, e)
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.running = true
	s.phase = "thinking"
	s.mu.Unlock()
	state, err := e.globalState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Sessions []proto.SessionItem `json:"sessions"`
		Tasks    int                 `json:"tasks"`
	}
	if err = json.Unmarshal(state.Data, &got); err != nil || len(got.Sessions) != 1 || !got.Sessions[0].Running {
		t.Fatal("global hydration", err)
	}
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}
