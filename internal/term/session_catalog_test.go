//go:build termtest

package term

import (
	"sync"
	"testing"
)

// The session picker shows turn counts, model details, and live updates
// from another client without closing and reopening the modal.
func TestSessionPickerShowsMetadataAndLiveUpdates(t *testing.T) {
	var mu sync.Mutex
	reply := "first response completed"

	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		mu.Lock()
		r := reply
		mu.Unlock()
		q.Text(r)
	}
	home := t.TempDir()

	// 1. Client B starts and creates a persisted nonempty session
	b := start(t, options{fake: f, home: home, cols: 120, rows: 30})
	b.Say("seed turn one")
	b.WaitFor("first response completed")
	b.Cmd("rename seeded-catalog-session")
	b.WaitFor("session renamed")

	// 2. Client A opens in the same home, opens /sessions
	a := start(t, options{fake: f, home: home, cols: 120, rows: 30})
	a.Cmd("sessions")
	a.WaitFor("seeded-catalog-session")
	a.WaitFor("2 turns")
	a.WaitFor("fake-model (fake, effort high)")

	// 3. Client B updates metadata while A has /sessions open:
	b.Cmd("rename renamed-catalog-session")
	b.WaitFor("session renamed")
	a.WaitFor("renamed-catalog-session")

	// 4. Client B runs another turn; A sees updated turn count live
	mu.Lock()
	reply = "second response completed"
	mu.Unlock()
	b.Say("seed turn two")
	b.WaitFor("second response completed")
	a.WaitFor("4 turns")
}
