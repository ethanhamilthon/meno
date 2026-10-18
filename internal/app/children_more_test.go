package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"meno/internal/agent"
	"meno/internal/llm"
)

// Block ends the child's run after the batch, resume continues the same session.
func TestSubagentBlockAndResume(t *testing.T) {
	cl := &routeClient{
		parent: []func(llm.Request) []llm.Event{
			tool("globalThis.id = agents.spawn({task: 'set up the db'}).id; id"),
			text("started"),
			tool("agents.status('X').status"),
		},
		child: []func(llm.Request) []llm.Event{
			tool("agents.block('which database?'); 'asked'"),
			text("unreachable before the reply"),
		},
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "go")
	waitUntil(t, a, "blocked", func() bool {
		l, _ := a.kids.store.List(a.st.Session.ID, false)
		return len(l) == 1 && l[0].Status == agent.ChildBlocked
	})
	l, _ := a.kids.store.List(a.st.Session.ID, false)
	id := l[0].ID
	if l[0].BlockReason != "which database?" {
		t.Fatalf("%+v", l[0])
	}
	cl.mu.Lock()
	cl.child = append(cl.child[:1], text("used sqlite"))
	cl.mu.Unlock()
	waitUntil(t, a, "the blocked event reaching the parent", func() bool { return len(cl.requests()) >= 4 })
	got := execJS(t, a, "agents.send('"+id+"', 'use sqlite').accepted", nil)
	if got != "true" {
		t.Fatalf("send: %q", got)
	}
	waitUntil(t, a, "completion after resume", func() bool {
		r, _ := a.kids.store.Get(a.st.Session.ID, id)
		return r.Status == agent.ChildCompleted
	})
	r, _ := a.kids.store.Get(a.st.Session.ID, id)
	if r.Gen != 2 || r.Result == nil || *r.Result != "used sqlite" {
		t.Fatalf("a resume is a new generation of the same child: %+v", r)
	}
	var seen bool
	for _, q := range cl.requests() {
		if strings.Contains(q.System, "You are a subagent") && strings.Contains(lastUser(q), "use sqlite") {
			seen = strings.Contains(lastUser(q), "source=\"parent\"")
		}
	}
	if !seen {
		t.Fatal("the reply must reach the child as a parent event")
	}
}

// A child has no way to ask the user, start tasks, spawn agents, rename or
// list sessions; cancelling one agent leaves the others alone.
func TestSubagentProhibitedBindingsAndCancel(t *testing.T) {
	cl := &routeClient{
		parent: []func(llm.Request) []llm.Event{
			tool("agents.spawn({task: 'probe'}).id + ',' + agents.spawn({task: 'sleeper'}).id"),
			text("two started"),
		},
		child: []func(llm.Request) []llm.Event{
			tool(`
				const r = [typeof tasks, typeof agents.spawn, typeof agents.report];
				for (const f of [() => askUser({questions: [{id: 'a', text: 't', type: 'text'}]}), () => session.rename('x y'), () => sessions.list(), () => sessions.search('x')]) {
					try { f(); r.push('allowed') } catch (e) { r.push('denied') }
				}
				r.join(',')
			`),
			text("probed"),
			text("never"),
		},
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "go")
	waitUntil(t, a, "both children to have run", func() bool {
		l, _ := a.kids.store.List(a.st.Session.ID, false)
		return len(l) == 2 && cl.childCalls() >= 2
	})
	var probe string
	for _, q := range cl.requests() {
		if strings.Contains(q.System, "You are a subagent") && lastToolResult(q) != "" {
			probe = lastToolResult(q)
		}
	}
	if probe != "undefined,undefined,function,denied,denied,denied,denied" {
		t.Fatalf("child bindings: %q", probe)
	}
	if got := execJS(t, a, "agents.send('nope', 'x')", nil); !strings.Contains(got, "not found") {
		t.Fatalf("unknown id: %q", got)
	}
}

// Usage of a child is counted apart; an unknown price is not shown as zero.
func TestSubagentUsageIsAccountedSeparatelyWithUnknownPrice(t *testing.T) {
	cl := &routeClient{
		parent: []func(llm.Request) []llm.Event{tool("agents.spawn({task: 'work'}).id"), text("ok"), text("done")},
		child:  []func(llm.Request) []llm.Event{text("finished")},
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "go")
	waitUntil(t, a, "completion", func() bool {
		l, _ := a.kids.store.List(a.st.Session.ID, false)
		return len(l) == 1 && l[0].Status == agent.ChildCompleted && len(cl.requests()) >= 4
	})
	l, _ := a.kids.store.List(a.st.Session.ID, false)
	got := execJS(t, a, "const u = agents.result('"+l[0].ID+"').usage; [u.in, u.out, u.cost, u.costIsLowerBound].join('|')", nil)
	if got != "100|10||true" {
		t.Fatalf("usage: %q", got)
	}
	a.onChildChanged(a.st.Session.ID)
	if a.st.Spent != 0 || !a.st.DelegatedUnpriced {
		t.Fatalf("delegated spend must stay apart from the parent's own: spent %v unpriced %v", a.st.Spent, a.st.DelegatedUnpriced)
	}
}

// :agents lists only working children; the user can message one through the
// steering queue with a <user-message> wrapper and without a new request.
func TestAgentsDialogListsOnlyWorkingAndMessagesThroughSteering(t *testing.T) {
	cl := &routeClient{
		parent: []func(llm.Request) []llm.Event{
			tool("agents.spawn({task: 'first, will block'}).id"), text("one started"), text("still waiting"),
		},
		child: []func(llm.Request) []llm.Event{
			tool("agents.block('need a decision'); 1"),
			text("decided by the user"),
		},
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "go")
	waitUntil(t, a, "blocked", func() bool {
		l, _ := a.kids.store.List(a.st.Session.ID, false)
		return len(l) == 1 && l[0].Status == agent.ChildBlocked
	})
	a.runCommand("agents")
	a.draw()
	text := screenText(scr)
	if !strings.Contains(text, "blocked") || !strings.Contains(text, "first, will block") {
		t.Fatalf("list:\n%s", text)
	}
	reqsBefore := a.st.RequestID
	a.onKey(keyK(tcell.KeyEnter)) // open the conversation
	typeRunes(a, "pick sqlite")
	a.onKey(keyK(tcell.KeyEnter))
	waitUntil(t, a, "the child to finish after the user's message", func() bool {
		l, _ := a.kids.store.List(a.st.Session.ID, false)
		return len(l) == 1 && l[0].Status == agent.ChildCompleted
	})
	var wrapped bool
	for _, q := range cl.requests() {
		if strings.Contains(q.System, "You are a subagent") && strings.Contains(lastUser(q), "<user-message") && strings.Contains(lastUser(q), "pick sqlite") {
			wrapped = true
		}
	}
	if !wrapped {
		t.Fatal("the child must receive the user's words in a <user-message> wrapper")
	}
	if a.st.RequestID != reqsBefore {
		t.Fatal("a direct message to a child must not start another request")
	}
	a.onKey(keyK(tcell.KeyEsc))
	a.onKey(keyK(tcell.KeyEsc))
	a.runCommand("agents")
	a.draw()
	if strings.Contains(screenText(scr), "first, will block") && !strings.Contains(screenText(scr), "no agents are working") {
		t.Fatalf("a finished child must not be listed:\n%s", screenText(scr))
	}
}
