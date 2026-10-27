package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/llm"
)

func okRuns(n int) *fakeProvider {
	f := &fakeProvider{}
	for i := 0; i < n; i++ {
		f.steps = append(f.steps, []llm.Event{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}})
	}
	return f
}

func apiCode(err error) string {
	if a, ok := err.(*daemon.APIError); ok {
		return a.Code
	}
	return ""
}

func create(t *testing.T, e *Engine, q api.SessionCreateRequest) (api.SessionItem, error) {
	t.Helper()
	if q.Cwd == "" {
		q.Cwd = t.TempDir()
	}
	q.ClientID = "c-" + t.Name() + q.Title + q.Model
	r, err := e.CreateSession(context.Background(), q)
	return r.Session, err
}

func send(t *testing.T, e *Engine, id string) {
	t.Helper()
	if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: "go", ClientID: "m-" + id}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running && s.persisted })
}

func TestCreateSessionModelEffortAndTitle(t *testing.T) {
	e, _ := fixture(t, okRuns(1))
	it, err := create(t, e, api.SessionCreateRequest{Model: "test/fake", Effort: "low", Title: "worker one"})
	if err != nil || it.Provider != "test" || it.Model != "fake" || it.Effort != "low" || it.Name != "worker one" {
		t.Fatalf("%+v %v", it, err)
	}
	// A bare model id belongs to the default provider.
	if it, err = create(t, e, api.SessionCreateRequest{Model: "fake", Title: "bare"}); err != nil || it.Provider != "test" {
		t.Fatalf("%+v %v", it, err)
	}
	for _, bad := range []api.SessionCreateRequest{
		{Model: "test/nope", Title: "a"},
		{Model: "other/fake", Title: "b"},
		{Model: "test/fake", Effort: "extreme", Title: "c"},
		{Model: "test/", Title: "d"},
	} {
		if _, err = create(t, e, bad); apiCode(err) != api.CodeBadRequest {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}

func TestSystemPromptIsFrozenInTheSessionAndReachesTheModel(t *testing.T) {
	f := okRuns(2)
	e, _ := fixture(t, f)
	dir := e.paths.SystemPromptsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "reviewer.md")
	if err := os.WriteFile(file, []byte("You review code.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	it, err := create(t, e, api.SessionCreateRequest{System: "reviewer", Title: "r"})
	if err != nil || it.System != "reviewer" {
		t.Fatalf("%+v %v", it, err)
	}
	// Editing the file later does not change a session that exists.
	if err = os.WriteFile(file, []byte("Something else."), 0o644); err != nil {
		t.Fatal(err)
	}
	send(t, e, it.ID)
	plain, err := create(t, e, api.SessionCreateRequest{System: "default", Title: "p"})
	if err != nil || plain.System != "" {
		t.Fatalf("%+v %v", plain, err)
	}
	send(t, e, plain.ID)
	if len(f.systems) != 2 || !strings.HasPrefix(f.systems[0], "You review code.\n\n") || !strings.HasPrefix(f.systems[1], "You are Meno, a coding agent.") {
		t.Fatalf("systems: %q", f.systems)
	}
	// The custom text replaces only the identity line.
	if !strings.Contains(f.systems[0], "<environment>") && !strings.Contains(f.systems[0], "environment") {
		t.Fatalf("the environment section is gone: %q", f.systems[0])
	}
	if _, err = create(t, e, api.SessionCreateRequest{System: "missing", Title: "m"}); apiCode(err) != api.CodeNotFound {
		t.Fatalf("missing prompt: %v", err)
	}
	if _, err = create(t, e, api.SessionCreateRequest{System: "../etc", Title: "x"}); apiCode(err) != api.CodeBadRequest {
		t.Fatalf("path as a name: %v", err)
	}
}

func TestSystemPromptSettingAppliesToNewSessionsAndFallsBack(t *testing.T) {
	e, _ := fixture(t, okRuns(1))
	store := config.NewStore(e.paths)
	st := config.DefaultSettings()
	st.Provider, st.Model, st.SystemPrompt = "test", "fake", "house"
	if err := store.SaveSettings(st); err != nil {
		t.Fatal(err)
	}
	// The file is missing: the built-in prompt is used, nothing fails.
	it, err := create(t, e, api.SessionCreateRequest{Title: "a"})
	if err != nil || it.System != "" {
		t.Fatalf("%+v %v", it, err)
	}
	if err = os.MkdirAll(e.paths.SystemPromptsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(e.paths.SystemPromptsDir(), "house.md"), []byte("House rules."), 0o644); err != nil {
		t.Fatal(err)
	}
	if it, err = create(t, e, api.SessionCreateRequest{Title: "b"}); err != nil || it.System != "house" {
		t.Fatalf("%+v %v", it, err)
	}
}

func TestForkOverridesAreCheckedBeforeAnythingIsCreated(t *testing.T) {
	e, _ := fixture(t, okRuns(1))
	src, err := create(t, e, api.SessionCreateRequest{Title: "source"})
	if err != nil {
		t.Fatal(err)
	}
	send(t, e, src.ID)
	before := len(listed(t, e))
	for _, bad := range []api.ForkRequest{
		{Model: "test/nope"}, {Effort: "extreme"}, {System: "missing"}, {Cwd: "/no/such/dir"},
	} {
		bad.ClientID = "f-bad"
		if _, err = e.Fork(context.Background(), src.ID, bad); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
	if n := len(listed(t, e)); n != before {
		t.Fatalf("a rejected fork left a session behind: %d -> %d", before, n)
	}
	dir := t.TempDir()
	r, err := e.Fork(context.Background(), src.ID, api.ForkRequest{ClientID: "f-ok", Effort: "low", Title: "child", Cwd: dir, NoAsk: true})
	if err != nil {
		t.Fatal(err)
	}
	it := r.Session
	if it.ForkOf != src.ID || it.Effort != "low" || it.Name != "child" || it.Cwd != dir || !it.NoAsk {
		t.Fatalf("%+v", it)
	}
	// The source is untouched and the fork keeps its model.
	if it.Model != "fake" || it.Provider != "test" {
		t.Fatalf("%+v", it)
	}
	if s, _ := e.Snapshot(context.Background(), src.ID); s.Session.Name != "source" || s.Session.Effort == "low" {
		t.Fatalf("source changed: %+v", s.Session)
	}
}
