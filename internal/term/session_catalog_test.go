//go:build termtest

package term

import "testing"

// The resume list shows turn counts and model details of a stored session.
func TestResumeListShowsMetadata(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("first response completed") }
	home := t.TempDir()

	b := start(t, options{fake: f, home: home, cols: 120, rows: 30})
	b.Say("seed turn one")
	b.WaitFor("first response completed")
	b.Cmd("rename seeded-catalog-session")
	b.WaitFor("session renamed")
	b.Cmd("q")
	if err := b.WaitExit(); err != nil {
		t.Fatal(err)
	}

	a := start(t, options{fake: f, home: home, cols: 120, rows: 30})
	a.Cmd("resume")
	a.WaitFor("seeded-catalog-session")
	a.WaitFor("2 turns")
	a.WaitFor("fake-model (fake, effort high)")
}
