package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/routing"
)

func TestRouteEffortOffUsesManualLevel(t *testing.T) {
	a, screen := newTestApp(t)
	defer screen.Fini()
	a.st.Providers = []llm.Provider{{Name: "p"}}
	a.st.Settings = config.DefaultSettings()
	var calls int
	a.client = func(llm.Provider) llm.Client {
		return routingClientFunc(func(context.Context, llm.Request, func(llm.Event)) error {
			calls++
			return nil
		})
	}
	got, notice, sticky := routeEffort(context.Background(), agent.Message{}, "p", "m", "medium", effortRouteConfig{
		settings: config.Settings{EffortRouter: "off"}, providers: a.st.Providers, sticky: a.st.EffortSticky, client: a.client,
	})
	if got != "medium" || notice != "" || sticky != nil || calls != 0 {
		t.Fatalf("route = %q, %q", got, notice)
	}
}

func TestRouteEffortAppliesAfterTwoRequests(t *testing.T) {
	a, screen := newTestApp(t)
	defer screen.Fini()
	client := &recClient{}
	a.client = func(llm.Provider) llm.Client { return client }
	a.st.Providers = []llm.Provider{{Name: "session", BaseURL: "http://x", Protocol: llm.ProtoChat}, {Name: "router", BaseURL: "http://x", Protocol: llm.ProtoChat}}
	a.st.Settings = config.DefaultSettings()
	a.st.Settings.EffortRouter = "auto"
	a.st.Settings.EffortRouterProvider = "router"
	a.st.Settings.EffortRouterModel = "small"
	client.mu.Lock()
	client.reqs = nil
	client.mu.Unlock()
	sticky := (*routing.Sticky)(nil)
	cfg := effortRouteConfig{settings: a.st.Settings, providers: a.st.Providers, allowed: []string{"low", "medium", "high"}, sticky: sticky, client: a.client}
	// Use the package's deterministic router hook through a minimal client response.
	responses := []string{"level: low\nreason: Simple edit.", "level: low\nreason: Simple edit."}
	var requestCount int
	cfg.client = func(p llm.Provider) llm.Client {
		if p.Name == "router" {
			return routingClientFunc(func(_ context.Context, req llm.Request, emit func(llm.Event)) error {
				requestCount++
				emit(llm.Event{Kind: llm.EvText, Text: responses[requestCount-1]})
				return nil
			})
		}
		return client
	}
	for i := 0; i < 2; i++ {
		var got, notice string
		got, notice, cfg.sticky = routeEffort(context.Background(), agent.Message{Text: "rename a variable"}, "session", "main", "high", cfg)
		if i == 0 && (got != "high" || notice != "") {
			t.Fatalf("first route = %q, %q", got, notice)
		}
		if i == 1 && (got != "low" || notice != "effort routing: high → low (Simple edit.)") {
			t.Fatalf("second route = %q, %q", got, notice)
		}
	}
	if requestCount != 2 {
		t.Fatalf("router calls %d", requestCount)
	}
}

type routingClientFunc func(context.Context, llm.Request, func(llm.Event)) error

func (f routingClientFunc) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	return f(ctx, req, emit)
}

func TestRouteEffortFailureFallsBackToManual(t *testing.T) {
	a, screen := newTestApp(t)
	defer screen.Fini()
	a.st.Settings = config.DefaultSettings()
	a.st.Settings.EffortRouter = "auto"
	a.st.Providers = []llm.Provider{{Name: "session", BaseURL: "http://x", Protocol: llm.ProtoChat}}
	var calls int
	a.client = func(llm.Provider) llm.Client {
		return routingClientFunc(func(context.Context, llm.Request, func(llm.Event)) error {
			calls++
			return errors.New("offline")
		})
	}
	got, notice, _ := routeEffort(context.Background(), agent.Message{}, "session", "main", "high", effortRouteConfig{
		settings: a.st.Settings, providers: a.st.Providers, allowed: []string{"low", "high"}, client: a.client,
	})
	if got != "high" || notice != "effort routing: using high (the router did not answer)" || calls != 1 {
		t.Fatalf("route = %q, %q; calls=%d", got, notice, calls)
	}
}

func TestRouteEffortFailureClearsPendingVerdict(t *testing.T) {
	settings := config.DefaultSettings()
	settings.EffortRouter = "auto"
	settings.EffortRouterModel = "router"
	settings.EffortRouterProvider = "router"
	provider := llm.Provider{Name: "router", Protocol: llm.ProtoChat, Models: map[string]llm.ModelSettings{"router": {Efforts: []string{"low", "high"}}}}
	calls := 0
	client := routingClientFunc(func(_ context.Context, _ llm.Request, emit func(llm.Event)) error {
		calls++
		if calls == 3 || calls == 4 {
			return errors.New("offline")
		}
		emit(llm.Event{Kind: llm.EvText, Text: "level: low\nreason: Small task."})
		return nil
	})
	cfg := effortRouteConfig{settings: settings, providers: []llm.Provider{provider}, allowed: []string{"low", "high"}, client: func(llm.Provider) llm.Client { return client }}
	route := func() (string, string) {
		level, notice, next := routeEffort(context.Background(), agent.Message{}, "router", "main", "high", cfg)
		cfg.sticky = next
		return level, notice
	}
	if level, notice := route(); level != "high" || notice != "" {
		t.Fatalf("first verdict = %q, %q", level, notice)
	}
	if level, notice := route(); level != "low" || notice != "effort routing: high → low (Small task.)" {
		t.Fatalf("second verdict = %q, %q", level, notice)
	}
	if level, notice := route(); level != "high" || notice == "" {
		t.Fatalf("router failure did not fall back: %q, %q", level, notice)
	}
	if level, notice := route(); level != "high" || notice == "" {
		t.Fatalf("repeated router failure changed manual effort: %q, %q", level, notice)
	}
}

func TestRouteEffortUpdatesRequestAndPromptWithoutChangingSession(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	a.st.Settings.EffortRouter = "auto"
	a.st.Settings.EffortRouterModel = "router"
	a.st.Settings.EffortRouterProvider = "openai"
	for i := range a.st.Providers {
		a.st.Providers[i].Models = map[string]llm.ModelSettings{
			"gpt-5.5": {Efforts: []string{"low", "high"}},
			"router":  {Efforts: []string{"low", "high"}},
		}
	}
	a.st.Session.Effort = "high"
	var routerCalls int
	var modelRequests []llm.Request
	a.client = func(llm.Provider) llm.Client {
		return routingClientFunc(func(_ context.Context, req llm.Request, emit func(llm.Event)) error {
			if !req.Tools {
				routerCalls++
				emit(llm.Event{Kind: llm.EvText, Text: "level: low\nreason: A simple task."})
				return nil
			}
			modelRequests = append(modelRequests, req)
			emit(llm.Event{Kind: llm.EvText, Text: "ok"})
			emit(llm.Event{Kind: llm.EvDone})
			return nil
		})
	}
	say(t, a, "one")
	if a.st.Session.Effort != "high" || len(modelRequests) != 1 || modelRequests[0].Effort != "high" {
		t.Fatalf("first request changed manual effort: session=%q requests=%+v", a.st.Session.Effort, modelRequests)
	}
	if got := a.lastNotice(); got != "" {
		t.Fatalf("unexpected first-request notice %q", got)
	}
	say(t, a, "two")
	if a.st.Session.Effort != "high" || routerCalls != 2 || len(modelRequests) != 2 {
		t.Fatalf("manual effort/session calls: %q router=%d model=%d", a.st.Session.Effort, routerCalls, len(modelRequests))
	}
	if modelRequests[1].Effort != "low" || !strings.Contains(modelRequests[1].System, "effort low") {
		t.Fatalf("routed effort missing from request prompt: %+v", modelRequests[1])
	}
	if strings.Contains(lastUser(modelRequests[1]), "<model-changed>") {
		t.Fatal("effort routing emitted a model change")
	}
	if got := a.lastNotice(); got != "effort routing: high → low (A simple task.)" {
		t.Fatalf("routing notice %q", got)
	}
}
