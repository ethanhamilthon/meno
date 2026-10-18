package app

import (
	"context"
	"testing"
)

// session.models are wired to the real store.
func TestJSHostSession(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "remember the milk")
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
	if got := run("session.models().errors && Array.isArray(session.models().models)"); got != "true" {
		t.Fatalf("models: %q", got)
	}
	if got := run("[typeof session.compact, typeof sessions, typeof state].join()"); got != "undefined,undefined,undefined" {
		t.Fatalf("removed globals are still there: %q", got)
	}
}
