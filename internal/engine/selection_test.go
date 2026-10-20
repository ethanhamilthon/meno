package engine

import (
	"context"
	"os"
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/proto"
)

// jsonOnlyEngine builds an engine whose settings live only in settings.json,
// like the daemon after the config service took over; the SQLite table is empty.
func jsonOnlyEngine(t *testing.T, f *fakeProvider, st config.Settings, providers ...llm.Provider) *Engine {
	t.Helper()
	e, _ := fixture(t, f)
	if err := config.SaveSettingsFile(e.paths.SettingsFile(), st); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(e.paths)
	if err := store.SaveSettings(config.DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	if providers != nil {
		if err := store.SaveProviders(providers); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func provider(name string, disabled bool) llm.Provider {
	return llm.Provider{Name: name, BaseURL: "http://127.0.0.1", Protocol: llm.ProtoResponses, Disabled: disabled, Models: map[string]llm.ModelSettings{"m": {Context: 1000}}}
}

func emptySession(t *testing.T, e *Engine, prov, model string) string {
	t.Helper()
	id := newSession(t, e)
	s, err := e.load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.session.Provider, s.session.Model = prov, model
	_ = e.save(s)
	s.mu.Unlock()
	return id
}

func done() *fakeProvider {
	return &fakeProvider{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
}

func TestJSONDefaultsReachNewSessions(t *testing.T) {
	st := config.DefaultSettings()
	st.Provider, st.Model, st.Effort = "json", "jm", "low"
	e := jsonOnlyEngine(t, done(), st, provider("json", false))
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	if s.session.Provider != "json" || s.session.Model != "jm" || s.session.Effort != "low" {
		t.Fatalf("new session %q %q %q", s.session.Provider, s.session.Model, s.session.Effort)
	}
}

func TestWhollyEmptySessionIsRepairedAndKeepsHistory(t *testing.T) {
	st := config.DefaultSettings()
	st.Provider, st.Model, st.Effort = "json", "jm", "low"
	f := done()
	e := jsonOnlyEngine(t, f, st, provider("json", false))
	id := emptySession(t, e, "", "")
	s, _ := e.load(context.Background(), id)
	s.mu.Lock()
	s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "earlier"})
	s.mu.Unlock()
	if _, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "again", ClientID: "m"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session.Provider != "json" || s.session.Model != "jm" || s.session.Effort != "low" {
		t.Fatalf("selection %q %q %q", s.session.Provider, s.session.Model, s.session.Effort)
	}
	if s.session.Snapshot()[0].Text != "earlier" {
		t.Fatal("history was changed")
	}
}

func TestNoUsableDefaultsRejectsWithoutUserTurn(t *testing.T) {
	st := config.DefaultSettings()
	e := jsonOnlyEngine(t, done(), st)
	id := emptySession(t, e, "", "")
	s, _ := e.load(context.Background(), id)
	before := len(s.session.Snapshot())
	_, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "hi", ClientID: "m"})
	if err == nil || !strings.Contains(err.Error(), "No model selected") {
		t.Fatalf("err %v", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running || len(s.session.Snapshot()) != before {
		t.Fatalf("running=%v turns=%d", s.running, len(s.session.Snapshot()))
	}
}

func TestPartialAndBrokenExplicitSelectionsAreNotReplaced(t *testing.T) {
	st := config.DefaultSettings()
	st.Provider, st.Model = "json", "jm"
	e := jsonOnlyEngine(t, done(), st, provider("json", false), provider("off", true))
	for _, c := range []struct{ prov, model, want string }{
		{"json", "", "No model selected"},
		{"", "m", "No model selected"},
		{"gone", "m", "not configured"},
		{"off", "m", "disabled"},
	} {
		id := emptySession(t, e, c.prov, c.model)
		_, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "hi", ClientID: "m" + c.prov + c.model})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%q/%q: err %v, want %q", c.prov, c.model, err, c.want)
		}
		s, _ := e.load(context.Background(), id)
		if s.session.Provider != c.prov || s.session.Model != c.model {
			t.Fatalf("%q/%q was changed to %q/%q", c.prov, c.model, s.session.Provider, s.session.Model)
		}
	}
}

func TestInvalidSettingsFileStopsTheRunAndKeepsTheLastSnapshot(t *testing.T) {
	st := config.DefaultSettings()
	st.Provider, st.Model = "json", "jm"
	e := jsonOnlyEngine(t, done(), st, provider("json", false))
	id := newSession(t, e)
	if err := os.WriteFile(e.paths.SettingsFile(), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := e.Message(context.Background(), id, proto.MessageRequest{Text: "hi", ClientID: "m"})
	if err == nil || !strings.Contains(err.Error(), "settings") {
		t.Fatalf("err %v", err)
	}
	if e.cfg().Model != "jm" {
		t.Fatalf("last valid snapshot was lost: %q", e.cfg().Model)
	}
}

func TestSettingsLoaderPrecedence(t *testing.T) {
	home := t.TempDir()
	store := config.NewStore(config.Paths{Home: home})
	legacy := config.DefaultSettings()
	legacy.Model = "legacy"
	if err := store.SaveSettings(legacy); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadCurrentSettings()
	if err != nil || got.Model != "legacy" {
		t.Fatalf("without a file: %q %v", got.Model, err)
	}
	file := config.DefaultSettings()
	file.Model = "file"
	if err := config.SaveSettingsFile(store.P.SettingsFile(), file); err != nil {
		t.Fatal(err)
	}
	if got, err = store.LoadCurrentSettings(); err != nil || got.Model != "file" {
		t.Fatalf("with a file: %q %v", got.Model, err)
	}
	if err = os.WriteFile(store.P.SettingsFile(), []byte("{x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = store.LoadCurrentSettings(); err == nil {
		t.Fatal("an invalid file must be an error, not a fallback to SQLite")
	}
}
