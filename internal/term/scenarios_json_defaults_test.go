//go:build termtest

package term

import "testing"

// Settings that live only in settings.json (the SQLite table is empty) must
// reach a new session: sending works with the default provider and model.
func TestJSONDefaultsSendWithEmptySQLiteSettings(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("json defaults work") }
	s := start(t, options{fake: f, jsonDefaults: true})
	s.Say("hello")
	s.WaitFor("json defaults work")
	if f.Requests() != 1 {
		t.Fatalf("requests: %d", f.Requests())
	}
}
