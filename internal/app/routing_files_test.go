package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
)

func TestRouteContextCountsShallowProjectEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"src", "main.go", ".git", ".notes"} {
		path := filepath.Join(dir, name)
		if name == "src" || strings.HasPrefix(name, ".") {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	settings := config.DefaultSettings()
	settings.EffortRouter = "auto"
	settings.EffortRouterModel = "router"
	settings.EffortRouterProvider = "router"
	provider := llm.Provider{Name: "router", Protocol: llm.ProtoChat, Models: map[string]llm.ModelSettings{"router": {Efforts: []string{"low", "high"}}}}
	var captured llm.Request
	client := routingClientFunc(func(_ context.Context, req llm.Request, emit func(llm.Event)) error {
		captured = req
		emit(llm.Event{Kind: llm.EvText, Text: "level: keep\nreason: Current effort is suitable."})
		return nil
	})
	_, _, _ = routeEffort(context.Background(), agent.Message{Text: "task"}, "router", "main", "high", effortRouteConfig{
		settings: settings, providers: []llm.Provider{provider}, cwd: dir, allowed: []string{"low", "high"},
		client: func(llm.Provider) llm.Client { return client },
	})
	if !strings.Contains(captured.Msgs[0].Text, "Project entries: 2") {
		t.Fatalf("unexpected project count: %q", captured.Msgs[0].Text)
	}
}

func TestCountProjectEntriesSkipsDotDirectories(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"main.go", "go.mod", ".git", ".meno", "folder"} {
		path := filepath.Join(dir, name)
		if name == ".git" || name == ".meno" || name == "folder" {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := countProjectEntries(dir); got != 3 {
		t.Fatalf("entry count %d", got)
	}
}
