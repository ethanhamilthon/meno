//go:build termtest

package term

import "testing"

// A second window receives activity without changing its unsent composer.
func TestReactiveGlobalActivityPreservesOtherWindowDraft(t *testing.T) {
	release := make(chan struct{})
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		q.TextPartial("reactive-running-marker")
		select {
		case <-release:
		case <-q.r.Context().Done():
			return
		}
		q.Text("reactive-completed-marker")
	}
	home := t.TempDir()
	a := start(t, options{fake: f, home: home, cols: 120, rows: 30})
	b := start(t, options{fake: f, home: home, cols: 120, rows: 30})
	b.Type("keep-background-window-draft")
	a.Say("start reactive work")
	a.WaitFor("reactive-running-marker")
	b.WaitFor("1 running")
	b.WaitFor("keep-background-window-draft")
	close(release)
	a.WaitFor("reactive-completed-marker")
	b.WaitGone("1 running")
	b.WaitFor("keep-background-window-draft")
}
