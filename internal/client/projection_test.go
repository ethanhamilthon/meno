package client

import (
	"meno/internal/api"
	"strconv"
	"testing"
)

func TestProjectionOnlyCopiesChangedFieldsAndAcceptedEffects(t *testing.T) {
	var m Mirror
	m.Load(api.Snapshot{Seq: 1, Turns: []api.Turn{{ID: "old"}}, Version: api.SessionVersion{BootID: "b", Revision: 1}})
	u, ok := m.ApplyUpdate(ev(2, api.EventTextDelta, api.TextDeltaData{Text: "a"}))
	if !ok || len(u.Values.Turns) != 0 || u.Changed&TurnsChanged != 0 {
		t.Fatalf("fragment copied history %+v", u)
	}
	m.Apply(ev(3, api.EventTextDelta, api.TextDeltaData{Text: "b"}))
	if u.Values.Live.Text != "a" {
		t.Fatal("published update mutated")
	}
	u, ok = m.ApplyUpdate(ev(4, api.EventDone, api.DoneData{}))
	if !ok || !u.Effects.Finished {
		t.Fatal("finish effect absent")
	}
	if _, ok = m.ApplyUpdate(ev(4, api.EventDone, api.DoneData{})); ok {
		t.Fatal("duplicate effect accepted")
	}
}

func TestProjectionChangesNeverClearRequiredRecovery(t *testing.T) {
	var m Mirror
	m.Load(api.Snapshot{Seq: 1, Session: api.SessionItem{ID: "s", Name: "old"}})
	metadata := api.ChangedData{Metadata: &api.SessionMetadata{Session: api.SessionItem{ID: "s", Name: "new"}, Provider: "p"}}
	m.Apply(ev(2, api.EventSessionChanged, metadata))
	if s := m.Snapshot(); s.NeedsResync || s.Session.Name != "new" || s.Provider != "p" || s.Effort != "" {
		t.Fatalf("metadata %+v", s)
	}
	m.Apply(ev(3, api.EventTasksChanged, api.TasksChangedData{Tasks: []api.Task{}}))
	if m.NeedsResync() {
		t.Fatal("empty task replacement requires snapshot")
	}
	m.Apply(ev(5, api.EventTextDelta, api.TextDeltaData{Text: "gap"}))
	m.Apply(ev(6, api.EventSessionChanged, metadata))
	if !m.NeedsResync() {
		t.Fatal("metadata cleared gap")
	}
	m.Load(api.Snapshot{Seq: 6})
	m.Apply(ev(7, api.EventSessionChanged, api.ChangedData{Fields: []string{"history"}, Metadata: metadata.Metadata}))
	if !m.NeedsResync() {
		t.Fatal("history replacement requires recovery")
	}
}

func TestProjectionEarlyCallAndFinalTurnUpdate(t *testing.T) {
	var m Mirror
	m.Load(api.Snapshot{Seq: 1})
	m.Apply(ev(2, api.EventCallDelta, api.CallDeltaData{Delta: "ec"}))
	m.Apply(ev(3, api.EventCallDelta, api.CallDeltaData{CallID: "c", Delta: "ho"}))
	m.Apply(ev(4, api.EventCallStart, api.CallStartData{CallID: "c", Command: "echo"}))
	if s := m.Snapshot(); len(s.Live.Calls) != 1 || s.Live.Calls[0].Command != "echo" || !s.StartedCalls["c"] {
		t.Fatalf("early call %+v", s.Live)
	}
	turn := api.Turn{ID: "a", Role: "assistant", Content: "ok"}
	m.Apply(ev(5, api.EventTurnAdded, api.TurnAddedData{Turn: turn}))
	turn.Run = &api.RunStats{Turns: 2}
	u, ok := m.ApplyUpdate(ev(6, api.EventTurnUpdated, api.TurnAddedData{Turn: turn}))
	if !ok || len(u.Values.Turns) != 1 || m.Snapshot().Turns[0].Run.Turns != 2 {
		t.Fatal("final stats lost")
	}
}

func TestProjectionRejectsRetiredBootAndOwnsNestedData(t *testing.T) {
	var m Mirror
	snap := api.Snapshot{Seq: 10, Version: api.SessionVersion{BootID: "old", Revision: 10}, Ask: &api.AskForm{Questions: []api.AskQuestion{{Options: []string{"one"}}}}}
	m.Load(snap)
	snap.Ask.Questions[0].Options[0] = "changed"
	if m.Snapshot().Ask.Questions[0].Options[0] != "one" {
		t.Fatal("nested snapshot alias")
	}
	m.Load(api.Snapshot{Seq: 1, Version: api.SessionVersion{BootID: "new", Revision: 1}})
	if _, ok := m.LoadUpdate(snap); ok {
		t.Fatal("retired boot accepted")
	}
}

func BenchmarkProjectionFragments(b *testing.B) {
	for _, n := range []int{1, 200} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			turns := make([]api.Turn, 2000)
			for i := range turns {
				turns[i] = api.Turn{ID: strconv.Itoa(i), Content: "history"}
			}
			var m Mirror
			m.Load(api.Snapshot{Turns: turns})
			events := make([]api.Event, n)
			for i := range events {
				events[i] = ev(0, api.EventTextDelta, api.TextDeltaData{Text: "x"})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.State.Live.Text = ""
				for _, e := range events {
					m.ApplyUpdate(e)
				}
			}
		})
	}
}
