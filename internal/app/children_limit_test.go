package app

import (
	"strings"
	"testing"

	"meno/internal/llm"
)

func parentCalls(c *routeClient) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pn
}

// The third spawn of a request is refused: the limit is the setting, and the
// refusal is the result of the call, so the agent sees it and can go on.
func TestSubagentSpawnStopsAtTheLimitOfTheRequest(t *testing.T) {
	cl := &routeClient{
		parent: []func(llm.Request) []llm.Event{
			tool("[agents.spawn({task: 'a'}).id, agents.spawn({task: 'b'}).id, (function(){ try { agents.spawn({task: 'c'}); return 'started'; } catch (e) { return String(e); } })()].join(' | ')"),
			text("done"),
		},
		child: []func(llm.Request) []llm.Event{text("one"), text("two")},
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.st.Settings.SubagentMaxSpawn = 2
	say(t, a, "go")
	waitUntil(t, a, "the parent answers", func() bool { return !a.st.Running && parentCalls(cl) >= 2 })
	var out string
	for _, tr := range a.st.Session.Snapshot() {
		if tr.Role == llm.RoleAssistant && strings.Contains(tr.Output, "subagent_max_spawn") {
			out = tr.Output
		}
	}
	if out == "" {
		t.Fatal("the refused third spawn must be in the result, with the limit named")
	}
	// Two ids and one error: the third spawn did not start.
	if parts := strings.Split(out, " | "); len(parts) != 3 || !strings.HasPrefix(parts[2], "GoError") || strings.HasPrefix(parts[2], "GoError: agents.spawn: task is required") {
		t.Fatalf("want two started, one refused: %q", out)
	}
}
