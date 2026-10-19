package routing

import (
	"context"
	"strings"
	"testing"

	"meno/internal/llm"
)

type routerScript struct {
	answer string
	got    llm.Request
}

func (s *routerScript) Stream(_ context.Context, req llm.Request, emit func(llm.Event)) error {
	s.got = req
	emit(llm.Event{Kind: llm.EvText, Text: s.answer})
	return nil
}

func TestModelRouterBuildsAllowedPromptAndParsesVerdict(t *testing.T) {
	client := &routerScript{answer: "level: high\nreason: It needs careful reasoning."}
	provider := llm.Provider{Protocol: llm.ProtoChat, Models: map[string]llm.ModelSettings{"router": {Efforts: []string{"low", "high"}}}}
	got, err := (ModelRouter{Client: client, Provider: provider, Model: "router"}).Route(context.Background(), Request{
		Current: "medium", Allowed: []string{"low", "high"},
		Context: Context{Text: "Change the architecture", Modes: []string{"goal"}, Attachments: 1, Files: 4},
	})
	if err != nil || got.Level != "high" {
		t.Fatalf("Route() = %+v, %v", got, err)
	}
	if !strings.Contains(client.got.System, "Allowed levels: low, high") || client.got.Effort != "low" || client.got.Tools {
		t.Fatalf("router request: %+v", client.got)
	}
	if !strings.Contains(client.got.Msgs[0].Text, "Change the architecture") {
		t.Fatalf("missing task context: %q", client.got.Msgs[0].Text)
	}
}

func TestModelRouterUsesLowestEffortInConfiguredOrder(t *testing.T) {
	client := &routerScript{answer: "level: low\nreason: Simple."}
	provider := llm.Provider{Models: map[string]llm.ModelSettings{"router": {Efforts: []string{"high", "low"}}}}
	_, err := (ModelRouter{Client: client, Provider: provider, Model: "router"}).Route(context.Background(), Request{Allowed: []string{"low", "high"}})
	if err != nil {
		t.Fatal(err)
	}
	if client.got.Effort != "high" {
		t.Fatalf("router effort %q", client.got.Effort)
	}
}

func TestModelRouterParsesKeep(t *testing.T) {
	client := &routerScript{answer: "level: keep\nreason: The current level is suitable."}
	got, err := (ModelRouter{Client: client, Provider: llm.Provider{}, Model: "router"}).Route(context.Background(), Request{Allowed: []string{"low", "high"}})
	if err != nil || got.Level != "" || got.Why == "" {
		t.Fatalf("Route() = %+v, %v", got, err)
	}
}

func TestModelRouterRejectsInvalidAnswer(t *testing.T) {
	for _, answer := range []string{"level: max\nreason: unsupported", "I think this is simple", "level: high\nreason: "} {
		client := &routerScript{answer: answer}
		if _, err := (ModelRouter{Client: client}).Route(context.Background(), Request{Allowed: []string{"low", "high"}}); err == nil {
			t.Fatalf("accepted %q", answer)
		}
	}
}
