package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/configsvc"
	"meno/internal/daemon"
	"meno/internal/db"
	"meno/internal/engine"
	"meno/internal/llm"
	"meno/internal/mcp"
)

// scripted answers each model request with the next list of events.
type scripted struct {
	steps [][]llm.Event
	n     int
}

func (f *scripted) Stream(_ context.Context, _ llm.Request, emit func(llm.Event)) error {
	if f.n >= len(f.steps) {
		emit(llm.Event{Kind: llm.EvText, Text: "done"})
		emit(llm.Event{Kind: llm.EvDone})
		return nil
	}
	for _, e := range f.steps[f.n] {
		emit(e)
	}
	f.n++
	return nil
}

func start(t *testing.T, f llm.Client) (home string, eng *engine.Engine, sess api.Snapshot, dapi *OwnerAPI) {
	h, e, s, a, _ := startRig(t, f)
	return h, e, s, a
}

func startRig(t *testing.T, f llm.Client) (home string, eng *engine.Engine, sess api.Snapshot, dapi *OwnerAPI, rpc *client.Client) {
	t.Helper()
	p := llm.Provider{Name: "test", BaseURL: "http://127.0.0.1", Protocol: llm.ProtoResponses, Models: map[string]llm.ModelSettings{"fake": {Context: 100000}}}
	return startRigWith(t, f, []llm.Provider{p}, func(context.Context, llm.Provider) ([]llm.Model, error) { return nil, nil })
}

func startRigWith(t *testing.T, f llm.Client, providers []llm.Provider, fetch func(context.Context, llm.Provider) ([]llm.Model, error)) (home string, eng *engine.Engine, sess api.Snapshot, dapi *OwnerAPI, rpc *client.Client) {
	t.Helper()
	home = t.TempDir()
	st := config.NewStore(config.Paths{Home: home})
	s := config.DefaultSettings()
	s.Provider, s.Model = "test", "fake"
	if err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveProviders(providers); err != nil {
		t.Fatal(err)
	}
	hub, global := daemon.NewHub("b", 2000), daemon.NewGlobalHub("b", 100)
	manager := mcp.NewManager()
	svc, err := configsvc.New(configsvc.Options{Paths: config.Paths{Home: home}, MCP: manager, FetchMetadata: func(context.Context) (map[string]llm.ModelInfo, error) { return nil, nil }, FetchModels: fetch})
	if err != nil {
		t.Fatal(err)
	}
	eng, err = engine.New(engine.Options{Resources: svc, MCP: manager, Home: home, EventHub: hub, GlobalHub: global, ProviderClient: func(llm.Provider) llm.Client { return f }})
	if err != nil {
		t.Fatal(err)
	}
	srv := daemon.New(daemon.Options{Backend: eng, Hub: hub, Global: global, Version: "test", OnStart: func() error { return svc.Start(context.Background()) }, OnStop: func() { eng.Close(); svc.Close() }})
	dial, err := srv.ServeMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Stop("test"); db.CloseAll() })
	made, err := eng.CreateSession(context.Background(), api.SessionCreateRequest{Cwd: t.TempDir(), ClientID: "c"})
	if err != nil {
		t.Fatal(err)
	}
	sess, err = eng.Snapshot(context.Background(), made.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	rpc = client.New(dial)
	dapi = New(home, rpc, sess.Session.ID, sess.AgentToken, "call1")
	return home, eng, sess, dapi, rpc
}

func runE2E(t *testing.T, dapi API, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, IO{In: strings.NewReader(""), Out: &out, Err: &errb, Dir: t.TempDir()}, dapi)
	return code, out.String(), errb.String()
}

func TestTasksThroughTheDaemon(t *testing.T) {
	_, _, _, dapi := start(t, &scripted{})
	code, out, errs := runE2E(t, dapi, "tasks", "start", "echo", "task-says-hi", "--json")
	if code != 0 {
		t.Fatalf("start: %d %s %s", code, out, errs)
	}
	tasks, err := dapi.TasksList(true)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("list: %v %v", tasks, err)
	}
	id := tasks[0].ID
	w, err := dapi.TaskWait(id, 5*time.Second)
	if err != nil || w.TimedOut || w.Task.Status == "running" {
		t.Fatalf("wait: %+v %v", w, err)
	}
	logs, err := dapi.TaskLogs(id, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	var all string
	for _, r := range logs.Records {
		all += r.Text
	}
	if !strings.Contains(all, "task-says-hi") {
		t.Fatalf("logs %+v", logs)
	}
}

func TestWrongTokenIsForbidden(t *testing.T) {
	home, _, sess, _, rpc := startRig(t, &scripted{})
	dapi := New(home, rpc, sess.Session.ID, "not-the-token", "c")
	_, err := dapi.TasksList(true)
	var ae *APIError
	if err == nil || !asAPI(err, &ae) || ae.Code != api.CodeForbidden {
		t.Fatalf("err %v", err)
	}
	code, _, errs := runE2E(t, dapi, "tasks", "list")
	if code != ExitForbidden {
		t.Fatalf("exit %d %s", code, errs)
	}
}

func asAPI(err error, target **APIError) bool {
	ae, ok := err.(*APIError)
	if ok {
		*target = ae
	}
	return ok
}

func TestAgentGoalAndViewWithoutActiveGoal(t *testing.T) {
	_, _, _, dapi := start(t, &scripted{})
	if _, err := dapi.Models("", false); err != nil {
		t.Fatal(err)
	}
}
