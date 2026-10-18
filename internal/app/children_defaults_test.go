package app

import (
	"strings"
	"testing"

	"meno/internal/config"
	"meno/internal/llm"
)

func TestSubagentDefaultsFallBackFieldByField(t *testing.T) {
	s := config.DefaultSettings()
	s.SubagentModel, s.SubagentEffort = "small", "low"
	p, m, e := subagentDefaults(s, map[string]any{}, "main", "big", "high")
	if p != "main" || m != "small" || e != "low" {
		t.Fatalf("settings defaults: %s %s %s", p, m, e)
	}
	// A field passed to agents.spawn keeps the inherited value here; the caller overwrites it.
	p, m, e = subagentDefaults(s, map[string]any{"model": "x"}, "main", "big", "high")
	if m != "big" || e != "low" {
		t.Fatalf("explicit model must not take the setting: %s %s %s", p, m, e)
	}
	p, m, e = subagentDefaults(config.DefaultSettings(), nil, "main", "big", "high")
	if p != "main" || m != "big" || e != "high" {
		t.Fatalf("inherit: %s %s %s", p, m, e)
	}
}

// The settings reach the child's actual request, not just the helper.
func TestSubagentSpawnUsesSettingsDefaults(t *testing.T) {
	cl := &routeClient{
		parent: []func(llm.Request) []llm.Event{
			tool("agents.spawn({task: 'work'}).id"),
			text("done"),
		},
		child: []func(llm.Request) []llm.Event{text("finished")},
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.st.Settings.SubagentModel = "small-model"
	a.st.Settings.SubagentEffort = "low"
	say(t, a, "go")
	waitUntil(t, a, "the child's first request", func() bool { return cl.childCalls() >= 1 })
	var req *llm.Request
	for _, q := range cl.requests() {
		if strings.Contains(q.System, "You are a subagent") {
			q := q
			req = &q
			break
		}
	}
	if req == nil || req.Model != "small-model" || req.Effort != "low" {
		t.Fatalf("child request %+v", req)
	}
}
