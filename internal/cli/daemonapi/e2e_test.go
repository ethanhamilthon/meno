package daemonapi_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"meno/internal/cli"
	"meno/internal/cli/daemonapi"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/daemon/cfgsvc"
	"meno/internal/db"
	"meno/internal/engine"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/proto"
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

func start(t *testing.T, f llm.Client) (home string, eng *engine.Engine, sess proto.Snapshot, api *daemonapi.API) {
	h, e, s, a, _ := startRig(t, f)
	return h, e, s, a
}

func startRig(t *testing.T, f llm.Client) (home string, eng *engine.Engine, sess proto.Snapshot, api *daemonapi.API, socket string) {
	t.Helper()
	home = t.TempDir()
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
	hub, global := daemon.NewHub("b", 2000), daemon.NewGlobalHub("b", 100)
	manager := mcp.NewManager()
	svc, err := cfgsvc.New(cfgsvc.Options{Paths: config.Paths{Home: home}, MCP: manager, FetchMetadata: func(context.Context) (map[string]llm.ModelInfo, error) { return nil, nil }, FetchModels: func(context.Context, llm.Provider) ([]llm.Model, error) { return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	eng, err = engine.New(engine.Options{Resources: svc, MCP: manager, Home: home, EventHub: hub, GlobalHub: global, ProviderClient: func(llm.Provider) llm.Client { return f }})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := daemon.New(daemon.Options{Home: home, Backend: eng, Hub: hub, Global: global, Version: "test", OnStart: svc.Start, OnStop: func() { eng.Close(); svc.Close() }})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Serve(ctx) }()
	t.Cleanup(func() { cancel(); srv.Stop("test"); eng.Close(); db.CloseAll() })
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(srv.Socket()); err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	made, err := eng.CreateSession(context.Background(), proto.SessionCreateRequest{Cwd: t.TempDir(), ClientID: "c"})
	if err != nil {
		t.Fatal(err)
	}
	sess, err = eng.Snapshot(context.Background(), made.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	api = daemonapi.New(home, srv.Socket(), sess.Session.ID, sess.AgentToken, "call1")
	return home, eng, sess, api, srv.Socket()
}

func run(t *testing.T, api cli.API, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := cli.Run(args, cli.IO{In: strings.NewReader(""), Out: &out, Err: &errb, Dir: t.TempDir()}, api)
	return code, out.String(), errb.String()
}

func TestTasksThroughTheDaemon(t *testing.T) {
	_, _, _, api := start(t, &scripted{})
	code, out, errs := run(t, api, "tasks", "start", "echo", "task-says-hi", "--json")
	if code != 0 {
		t.Fatalf("start: %d %s %s", code, out, errs)
	}
	tasks, err := api.TasksList(true)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("list: %v %v", tasks, err)
	}
	id := tasks[0].ID
	w, err := api.TaskWait(id, 5*time.Second)
	if err != nil || w.TimedOut || w.Task.Status == "running" {
		t.Fatalf("wait: %+v %v", w, err)
	}
	logs, err := api.TaskLogs(id, 0, 50)
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
	home, _, sess, _, sock := startRig(t, &scripted{})
	api := daemonapi.New(home, sock, sess.Session.ID, "not-the-token", "c")
	_, err := api.TasksList(true)
	var ae *cli.APIError
	if err == nil || !asAPI(err, &ae) || ae.Code != proto.CodeForbidden {
		t.Fatalf("err %v", err)
	}
	code, _, errs := run(t, api, "tasks", "list")
	if code != cli.ExitForbidden {
		t.Fatalf("exit %d %s", code, errs)
	}
}

func asAPI(err error, target **cli.APIError) bool {
	ae, ok := err.(*cli.APIError)
	if ok {
		*target = ae
	}
	return ok
}

func TestSessionsListAndSend(t *testing.T) {
	_, _, sess, api := start(t, &scripted{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "pong"}, {Kind: llm.EvDone}}}})
	items, err := api.Sessions("")
	if err != nil || len(items) != 0 {
		t.Fatalf("a session without a message must not be listed: %v %v", items, err)
	}
	res, err := api.Send(cli.SendRequest{Text: "ping", SessionID: sess.Session.ID})
	if err != nil || !res.Done || !strings.Contains(res.Text, "pong") {
		t.Fatalf("send %+v %v", res, err)
	}
	items, err = api.Sessions("")
	if err != nil || len(items) != 1 || items[0].ID != sess.Session.ID {
		t.Fatalf("sessions after the first message %v %v", items, err)
	}
}

func TestAgentGoalAndViewWithoutActiveGoal(t *testing.T) {
	_, _, _, api := start(t, &scripted{})
	if _, err := api.Models("", false); err != nil {
		t.Fatal(err)
	}
}

func TestSendNewCreatesExactlyOneListedSession(t *testing.T) {
	_, _, _, api := start(t, &scripted{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "pong"}, {Kind: llm.EvDone}}}})
	res, err := api.Send(cli.SendRequest{Text: "ping", New: true})
	if err != nil || res.SessionID == "" || !res.Done {
		t.Fatalf("send %+v %v", res, err)
	}
	items, err := api.Sessions("")
	if err != nil || len(items) != 1 || items[0].ID != res.SessionID {
		t.Fatalf("sessions %v %v", items, err)
	}
}
