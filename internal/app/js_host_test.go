package app

import (
	"context"
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/llm"
)

// The js globals state, session and sessions are wired to the real stores.
func TestJSHostStateSessionAndRetrieval(t *testing.T) {
	cl := &recClient{}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "remember the milk")
	id := a.st.Session.ID
	run := func(code string) string {
		t.Helper()
		var res string
		done := make(chan struct{})
		go func() {
			r := a.st.VM.Exec(context.Background(), code)
			res = r.Output + r.Value + r.Err
			close(done)
		}()
		for {
			select {
			case <-done:
				return res
			case e := <-a.events:
				a.handle(e)
			}
		}
	}
	if got := run("state.set('plan', {step: 1}); state.get('plan').step"); got != "1" {
		t.Fatalf("state: %q", got)
	}
	kv, _ := agent.OpenKV(paths.Home, id, agent.NSState)
	if es, _ := kv.Entries(); len(es) != 1 || es[0].JSON != `{"step":1}` {
		t.Fatalf("state is not in the database: %+v", es)
	}
	if got := run("session.rename('Buy milk soon')"); got != "Buy milk soon" {
		t.Fatalf("rename: %q", got)
	}
	if a.st.Session.Title != "Buy milk soon" {
		t.Fatalf("title not applied to the live session: %q", a.st.Session.Title)
	}
	if got := run("session.rename('this is far too many words for a name')"); !strings.Contains(got, "at most 6 words") {
		t.Fatalf("long name: %q", got)
	}
	if got := run("sessions.search('milk').items.length > 0 && sessions.list().items[0].id === " + "'" + id + "'"); got != "true" {
		t.Fatalf("retrieval: %q", got)
	}
	if got := run("sessions.read(sessions.list().items[0].id).records[0].text"); !strings.Contains(got, "remember the milk") {
		t.Fatalf("read: %q", got)
	}
	if got := run("session.models().errors && Array.isArray(session.models().models)"); got != "true" {
		t.Fatalf("models: %q", got)
	}
	if got := run("session.compact('s', -1)"); !strings.Contains(got, "non-negative") {
		t.Fatalf("compact validation: %q", got)
	}
	_ = llm.RoleUser
}
