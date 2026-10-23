package engine

import (
	"context"
	"encoding/json"
	"testing"

	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
)

func newTestEngine(t *testing.T, home string) *Engine {
	t.Helper()
	store := config.NewStore(config.Paths{Home: home})
	st := config.DefaultSettings()
	st.Provider = "test"
	st.Model = "fake"
	if err := store.SaveSettings(st); err != nil {
		t.Fatal(err)
	}
	p := llm.Provider{
		Name:     "custom-prov",
		BaseURL:  "http://127.0.0.1",
		Protocol: llm.ProtoResponses,
		Models:   map[string]llm.ModelSettings{"custom-model": {Context: 100000}},
	}
	if err := store.SaveProviders([]llm.Provider{p}); err != nil {
		t.Fatal(err)
	}
	hub := daemon.NewHub("boot", 2000)
	global := daemon.NewGlobalHub("boot", 1000)
	f := &fakeProvider{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
	e, err := New(Options{MCP: mcp.NewManager(), Home: home, EventHub: hub, GlobalHub: global, ProviderClient: func(llm.Provider) llm.Client { return f }})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestSessionListMetadataUnloadedLoadedRestartCompactedAndZero(t *testing.T) {
	home := t.TempDir()
	e := newTestEngine(t, home)
	defer func() { e.Close(); db.CloseAll() }()

	r, err := e.CreateSession(context.Background(), api.SessionCreateRequest{Cwd: t.TempDir(), ClientID: "c1", Model: "custom-prov/custom-model"})
	if err != nil {
		t.Fatal(err)
	}
	id := r.Session.ID
	prov := "custom-prov"
	eff := "high"
	name := "Metadata session"
	if _, err := e.PatchSession(context.Background(), id, api.PatchSessionRequest{Provider: &prov, Effort: &eff, Name: &name}); err != nil {
		t.Fatal(err)
	}

	// Persist 3 turns
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "turn 1"})
	s.session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "turn 2"})
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "turn 3"})
	if err := e.save(s); err != nil {
		s.mu.Unlock()
		t.Fatal(err)
	}
	s.mu.Unlock()

	// 1. Verify when unloaded in memory
	e.mu.Lock()
	delete(e.sessions, id)
	e.mu.Unlock()

	list, err := e.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list.Sessions))
	}
	it := list.Sessions[0]
	if it.TurnCount != 3 || it.Provider != "custom-prov" || it.Model != "custom-model" || it.Effort != "high" || it.Name != "Metadata session" {
		t.Fatalf("unloaded list item mismatch: %+v", it)
	}

	// Verify globalState matches
	res, err := e.globalState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var gState struct {
		Sessions []api.SessionItem `json:"sessions"`
		Tasks    int               `json:"tasks"`
	}
	if err := json.Unmarshal(res.Data, &gState); err != nil {
		t.Fatal(err)
	}
	if len(gState.Sessions) != 1 || gState.Sessions[0].TurnCount != 3 || gState.Sessions[0].Provider != "custom-prov" || gState.Sessions[0].Effort != "high" {
		t.Fatalf("unloaded globalState mismatch: %+v", gState.Sessions)
	}

	// 2. Verify when loaded in memory
	snap, err := e.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TurnCount != 3 || snap.Session.TurnCount != 3 || snap.Session.Provider != "custom-prov" || snap.Session.Effort != "high" {
		t.Fatalf("loaded snapshot mismatch: snap=%+v session=%+v", snap, snap.Session)
	}
	listLoaded, err := e.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if listLoaded.Sessions[0].TurnCount != snap.TurnCount {
		t.Fatalf("list count (%d) != snapshot count (%d)", listLoaded.Sessions[0].TurnCount, snap.TurnCount)
	}

	// 3. Verify after engine restart
	e.Close()
	db.CloseAll()
	e2 := newTestEngine(t, home)
	defer func() { e2.Close(); db.CloseAll() }()

	list2, err := e2.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list2.Sessions) != 1 || list2.Sessions[0].TurnCount != 3 || list2.Sessions[0].Provider != "custom-prov" || list2.Sessions[0].Effort != "high" {
		t.Fatalf("after restart list mismatch: %+v", list2.Sessions)
	}

	// 4. Verify compacted history: count equals active displayed history, not all archived rows
	s2, err := e2.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s2.mu.Lock()
	all := s2.session.Snapshot()
	s2.session.Replace(all[2:]) // keep only 1 turn active
	s2.session.Summary = "compacted summary"
	if err := e2.save(s2); err != nil {
		s2.mu.Unlock()
		t.Fatal(err)
	}
	s2.mu.Unlock()

	compactList, err := e2.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	compactSnap, err := e2.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if compactList.Sessions[0].TurnCount != 1 || compactSnap.TurnCount != 1 || compactSnap.Session.TurnCount != 1 {
		t.Fatalf("compacted turn count: list=%d snap=%d session=%d", compactList.Sessions[0].TurnCount, compactSnap.TurnCount, compactSnap.Session.TurnCount)
	}

	// 5. Genuine zero-active-turn summary-only session
	rZero, err := e2.CreateSession(context.Background(), api.SessionCreateRequest{Cwd: t.TempDir(), ClientID: "c0"})
	if err != nil {
		t.Fatal(err)
	}
	zeroID := rZero.Session.ID
	sZero, err := e2.load(context.Background(), zeroID)
	if err != nil {
		t.Fatal(err)
	}
	sZero.mu.Lock()
	sZero.session.Summary = "initial recap"
	sZero.session.Title = "Summary only"
	if err := e2.save(sZero); err != nil {
		sZero.mu.Unlock()
		t.Fatal(err)
	}
	sZero.mu.Unlock()

	zeroList, err := e2.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var zeroItem api.SessionItem
	found := false
	for _, it := range zeroList.Sessions {
		if it.ID == zeroID {
			zeroItem = it
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("zero-turn summary session not found in list: %+v", zeroList.Sessions)
	}
	if zeroItem.TurnCount != 0 {
		t.Fatalf("expected turn count 0, got %d", zeroItem.TurnCount)
	}
	zeroSnap, err := e2.Snapshot(context.Background(), zeroID)
	if err != nil {
		t.Fatal(err)
	}
	if zeroSnap.TurnCount != 0 || zeroSnap.Session.TurnCount != 0 {
		t.Fatalf("expected 0 turn count in snap: snap=%d session=%d", zeroSnap.TurnCount, zeroSnap.Session.TurnCount)
	}
}
