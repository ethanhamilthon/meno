package client

import (
	"meno/internal/api"
	"testing"
)

func TestCompletionCountedProjection(t *testing.T) {
	for _, kind := range []string{api.EventDone, api.EventAborted} {
		for _, counted := range []bool{false, true} {
			var m Mirror
			m.Load(api.Snapshot{})
			u, ok := m.ApplyUpdate(ev(1, kind, api.DoneData{Counted: counted}))
			if !ok || !u.Effects.Finished || u.Effects.Counted != counted {
				t.Fatalf("%s counted=%v: %+v", kind, counted, u.Effects)
			}
		}
	}
}
