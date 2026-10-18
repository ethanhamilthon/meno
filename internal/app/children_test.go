package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
)

// routeClient answers parent and child requests by who is asking: a request
// whose system prompt says "You are a subagent" is a child's.
type routeClient struct {
	mu     sync.Mutex
	parent []func(llm.Request) []llm.Event
	child  []func(llm.Request) []llm.Event
	pn, cn int
	reqs   []llm.Request
}

func (c *routeClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	var evs []llm.Event
	if strings.Contains(req.System, "You are a subagent") {
		if c.cn < len(c.child) {
			evs = c.child[c.cn](req)
		}
		c.cn++
	} else {
		if c.pn < len(c.parent) {
			evs = c.parent[c.pn](req)
		}
		c.pn++
	}
	child := strings.Contains(req.System, "You are a subagent")
	c.mu.Unlock()
	for _, e := range evs {
		emit(e)
	}
	done := llm.Event{Kind: llm.EvDone}
	if child {
		done.Usage = llm.Usage{In: 100, Out: 10}
	}
	emit(done)
	return nil
}

func (c *routeClient) childCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cn
}

func (c *routeClient) requests() []llm.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]llm.Request(nil), c.reqs...)
}

func text(s string) func(llm.Request) []llm.Event {
	return func(llm.Request) []llm.Event { return []llm.Event{{Kind: llm.EvText, Text: s}} }
}

func tool(code string) func(llm.Request) []llm.Event {
	return func(llm.Request) []llm.Event {
		return []llm.Event{{Kind: llm.EvTool, CallID: agent.NewID(), Code: code}}
	}
}

func waitUntil(t *testing.T, a *App, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second) // CI runners with -race can be slow
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		select {
		case e := <-a.events:
			a.handle(e)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func lastToolResult(r llm.Request) string {
	for i := len(r.Msgs) - 1; i >= 0; i-- {
		if r.Msgs[i].Role == llm.RoleTool {
			return r.Msgs[i].Text
		}
	}
	return ""
}

// A subagent runs on its own, completes, and its completion wakes the idle
// parent as an event; the child is internal-only.
func TestSubagentSpawnCompleteWakesParent(t *testing.T) {
	cl := &routeClient{
		parent: []func(llm.Request) []llm.Event{
			tool("agents.spawn({task: 'Count the files in /tmp and report the number.'}).id"),
			text("spawned, waiting"),
			text("the agent finished"),
		},
		child: []func(llm.Request) []llm.Event{text("There are 7 files.")},
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "delegate")
	waitUntil(t, a, "the wake-up request", func() bool { return len(cl.requests()) >= 4 && !a.st.Running })
	var parentReqs []llm.Request
	for _, r := range cl.requests() {
		if !strings.Contains(r.System, "You are a subagent") {
			parentReqs = append(parentReqs, r)
		}
	}
	if len(parentReqs) != 3 {
		t.Fatalf("parent requests: %d", len(parentReqs))
	}
	wake := lastUser(parentReqs[2])
	if !strings.Contains(wake, "source=\"child\"") || !strings.Contains(wake, "kind=\"completed\"") || !strings.Contains(wake, "There are 7 files.") {
		t.Fatalf("the parent did not get the completion: %q", wake)
	}
	// The child saw only its explicit task, not the parent's conversation.
	for _, r := range cl.requests() {
		if strings.Contains(r.System, "You are a subagent") {
			if first := r.Msgs[0].Text; !strings.Contains(first, "Count the files") || strings.Contains(first, "delegate") {
				t.Fatalf("child context: %q", first)
			}
		}
	}
	list, _ := a.kids.store.List(a.st.Session.ID, false)
	if len(list) != 1 || list[0].Status != agent.ChildCompleted || list[0].Result == nil || *list[0].Result != "There are 7 files." {
		t.Fatalf("%+v", list)
	}
	// Not in Sessions; the parent gets the result through its ID.
	infos, _ := agent.ListSessions(a.paths.Home)
	for _, s := range infos {
		if s.ID == list[0].ID {
			t.Fatal("a child must not appear in Sessions")
		}
	}
	got := execJS(t, a, "agents.result('"+list[0].ID+"').result", nil)
	if got != "There are 7 files." {
		t.Fatalf("parent access: %q", got)
	}
}
