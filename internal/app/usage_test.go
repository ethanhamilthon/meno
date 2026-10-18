package app

import (
	"fmt"
	"math"
	"meno/internal/ui"
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/llm"
)

func TestRequestPricingAvailability(t *testing.T) {
	zero := 0.0
	for _, tc := range []struct {
		name     string
		info     llm.ModelInfo
		u        llm.Usage
		cost     float64
		unpriced bool
	}{
		{"legacy-no-write", llm.ModelInfo{Input: 2, Output: 10}, llm.Usage{In: 100, Out: 10, Cached: 50}, 0.0003, false},
		{"manual-free", llm.ModelInfo{}, llm.Usage{In: 100, Out: 10}, 0, false},
		{"unknown-write", llm.ModelInfo{Input: 2, Output: 10}, llm.Usage{In: 100, Out: 10, CacheWrite: 50}, 0, true},
		{"unknown-write-only", llm.ModelInfo{}, llm.Usage{CacheWrite: 50}, 0, true},
		{"free-write", llm.ModelInfo{Input: 2, Output: 10, CacheWrite: &zero}, llm.Usage{In: 100, Out: 10, CacheWrite: 50}, 0.0002, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, screen := newTestApp(t)
			defer screen.Fini()
			a.st.Pricing = map[string]llm.ModelInfo{"openai/test": tc.info}
			a.st.Spent = 1
			a.onAgent(agent.Ev{Kind: agent.EvUsage, Provider: llm.Provider{Name: "openai"}, Model: "test", Usage: tc.u})
			if a.st.Unpriced != tc.unpriced || math.Abs(a.st.Spent-(1+tc.cost)) > 1e-10 {
				t.Fatalf("unpriced = %v, spent = %g", a.st.Unpriced, a.st.Spent)
			}
			if len(a.st.Session.Requests) != 1 {
				t.Fatal("missing request ledger entry")
			}
			entry := a.st.Session.Requests[0]
			if entry.Usage != tc.u || (entry.Cost == nil) != tc.unpriced {
				t.Fatalf("record %+v", entry)
			}
			if entry.Cost != nil && math.Abs(*entry.Cost-tc.cost) > 1e-10 {
				t.Fatalf("cost = %g, want %g", *entry.Cost, tc.cost)
			}
		})
	}
}

func TestRequestLedgerAndLastUsage(t *testing.T) {
	a, screen := newTestApp(t)
	defer screen.Fini()
	a.st.Providers = []llm.Provider{{Name: "openai"}}
	a.st.Settings.Provider = "openai"
	a.st.Settings.Model = "selected-other-model"
	read, write := 0.4, 5.0
	a.st.Pricing = map[string]llm.ModelInfo{"openai/actual": {Input: 4, Output: 20, CacheRead: &read, CacheWrite: &write}}
	first := llm.Usage{In: 1000, Out: 100, Cached: 600, CacheWrite: 300, CacheReported: true}
	last := llm.Usage{In: 200, Out: 20, Cached: 100, CacheWrite: 50, CacheReported: true}
	for _, u := range []llm.Usage{first, last} {
		a.onAgent(agent.Ev{Kind: agent.EvUsage, Provider: llm.Provider{Name: "openai"}, Model: "actual", Usage: u})
	}
	if a.st.LastUsage != last || a.st.Usage.In != 1200 || a.st.Usage.CacheWrite != 350 || a.st.Context != 220 {
		t.Fatalf("stats %+v", a.st)
	}
	if a.st.Unpriced || math.Abs(a.st.Spent-0.00503) > 1e-10 {
		t.Fatalf("cost %g", a.st.Spent)
	}
	text := a.statsText()
	if !strings.Contains(text, "last in 200") || !strings.Contains(text, "read 100 (50% of input) · write 50") || !strings.Contains(text, "read 700 (58% of input)") || !strings.Contains(text, "session in 1200") {
		t.Fatal(text)
	}
	a.onTurnDone(nil)
	id := a.st.Session.ID
	a.runCommand("new")
	if a.st.LastUsage != (llm.Usage{}) || len(a.st.Session.Requests) != 0 {
		t.Fatal("new session retained usage")
	}
	a.dropRuntime(a.rts[id])
	a.openSession(id)
	if a.st.LastUsage != last || len(a.st.Session.Requests) != 2 {
		t.Fatal("resume lost ledger")
	}
	entry := a.st.Session.Requests[0]
	if entry.Model != "actual" || entry.Provider != "openai" || entry.Usage != first || entry.Cost == nil || entry.Time.IsZero() {
		t.Fatalf("record %+v", entry)
	}
	a.saveSession()
	saved, err := agent.LoadSession(a.paths.Home, id)
	if err != nil || len(saved.Requests) != 2 || saved.Metrics.LastUsage != last {
		t.Fatalf("clear lost ledger: %+v %v", saved, err)
	}
	a.onAgent(agent.Ev{Kind: agent.EvUsage, Provider: llm.Provider{Name: "unknown"}, Model: "unknown", Usage: last})
	if !a.st.Unpriced || a.st.Session.Requests[2].Cost != nil {
		t.Fatal("unknown price recorded as free")
	}
}

func TestStatusUsageIcons(t *testing.T) {
	a, screen := newTestApp(t)
	defer screen.Fini()
	a.st.Providers = []llm.Provider{{Name: "openai"}}
	a.st.Settings.Provider, a.st.Settings.Model = "openai", "test"
	a.st.Pricing = map[string]llm.ModelInfo{"openai/test": {Context: 128000}}
	a.st.Usage = llm.Usage{In: 12000, Out: 2000, Cached: 10000, CacheReported: true}
	a.st.LastUsage = llm.Usage{In: 200, Cached: 50, CacheReported: true}
	a.st.Context, a.st.Spent = 4000, 1.25
	want := "12k/2k ↺25%/83% ◫ 4k/128k $1.25"
	if got := a.statusUsageText(); got != want {
		t.Fatalf("%q, want %q", got, want)
	}
	screen.SetSize(160, 1)
	a.drawStatus(ui.Canvas{S: screen}, 0, 0, 160)
	if got := screenText(screen); !strings.Contains(got, want) {
		t.Fatal("status missing metrics:", got)
	}
	a.st.Pricing = nil
	a.st.Unpriced = true
	a.st.Usage = llm.Usage{In: 100, Cached: 200, CacheReported: true}
	if got := a.statusUsageText(); !strings.Contains(got, "↺25%/100%") || !strings.Contains(got, "4k/?") || !strings.Contains(got, "$1.25+?") {
		t.Fatal(got)
	}
	a.st.Usage = llm.Usage{Cached: 100, CacheReported: true}
	a.st.LastUsage = llm.Usage{In: 100, Cached: 100, CacheReported: true}
	if got := a.statusUsageText(); !strings.Contains(got, "↺100%/0%") {
		t.Fatal(got)
	}
}

func TestCatalogPickerAndCache(t *testing.T) {
	a, screen := newTestApp(t)
	defer screen.Fini()
	a.st.Providers = []llm.Provider{{Name: "personal", BaseURL: "https://api.openai.com/v1"}}
	a.st.Pricing = map[string]llm.ModelInfo{"openai/catalog-only": {Context: 128000}, "openai/shared": {}, "openrouter/openai/other": {}}
	a.st.Models["personal"] = []llm.Model{{ID: "shared", Provider: "personal"}, {ID: "live-only", Provider: "personal"}}
	want := []string{"catalog-only", "live-only", "shared"}
	models := a.allModels()
	if len(models) != len(want) {
		t.Fatal(models)
	}
	for i, id := range want {
		if models[i].ID != id || models[i].Provider != "personal" {
			t.Fatal(models)
		}
	}
	a.st.Query = []rune("catalog-only")
	if models := a.modelList(); len(models) != 1 || models[0].ID != "catalog-only" {
		t.Fatal(models)
	}
	if err := a.store.SaveCatalog(a.st.Pricing); err != nil {
		t.Fatal(err)
	}
	a.st.Pricing = nil
	a.loadMetadataCache()
	a.st.Models["personal"] = nil
	if models := a.allModels(); len(models) != 2 {
		t.Fatal("offline cache unavailable:", models)
	}
	a.onModelsLoaded(modelsLoaded{provider: "personal", err: fmt.Errorf("offline")})
	if models := a.allModels(); len(models) != 2 {
		t.Fatal("provider failure hid catalog:", models)
	}
}
