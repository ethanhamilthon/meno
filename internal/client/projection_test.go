package client

import (
	"meno/internal/proto"
	"strconv"
	"testing"
)

func TestProjectionOnlyCopiesChangedFieldsAndAcceptedEffects(t *testing.T) {
	var m Mirror
	m.Load(proto.Snapshot{Seq: 1, Turns: []proto.Turn{{ID: "old"}}, Version: proto.SessionVersion{BootID: "b", Revision: 1}})
	u, ok := m.ApplyUpdate(ev(2, proto.EventTextDelta, proto.TextDeltaData{Text: "a"}))
	if !ok || len(u.Values.Turns) != 0 || u.Changed&TurnsChanged != 0 {
		t.Fatalf("fragment copied history %+v", u)
	}
	m.Apply(ev(3, proto.EventTextDelta, proto.TextDeltaData{Text: "b"}))
	if u.Values.Live.Text != "a" {
		t.Fatal("published update mutated")
	}
	u, ok = m.ApplyUpdate(ev(4, proto.EventDone, proto.DoneData{}))
	if !ok || !u.Effects.Finished {
		t.Fatal("finish effect absent")
	}
	if _, ok = m.ApplyUpdate(ev(4, proto.EventDone, proto.DoneData{})); ok {
		t.Fatal("duplicate effect accepted")
	}
}

func TestProjectionChangesNeverClearRequiredRecovery(t *testing.T) {
	var m Mirror
	m.Load(proto.Snapshot{Seq: 1, Session: proto.SessionItem{ID: "s", Name: "old"}})
	metadata := proto.ChangedData{Metadata: &proto.SessionMetadata{Session: proto.SessionItem{ID: "s", Name: "new"}, Provider: "p"}}
	m.Apply(ev(2, proto.EventSessionChanged, metadata))
	if s := m.Snapshot(); s.NeedsResync || s.Session.Name != "new" || s.Provider != "p" || s.Effort != "" {
		t.Fatalf("metadata %+v", s)
	}
	m.Apply(ev(3, proto.EventTasksChanged, proto.TasksChangedData{Tasks: []proto.Task{}}))
	if m.NeedsResync() {
		t.Fatal("empty task replacement requires snapshot")
	}
	m.Apply(ev(5, proto.EventTextDelta, proto.TextDeltaData{Text: "gap"}))
	m.Apply(ev(6, proto.EventSessionChanged, metadata))
	if !m.NeedsResync() {
		t.Fatal("metadata cleared gap")
	}
	m.Load(proto.Snapshot{Seq: 6})
	m.Apply(ev(7, proto.EventSessionChanged, proto.ChangedData{Fields: []string{"history"}, Metadata: metadata.Metadata}))
	if !m.NeedsResync() {
		t.Fatal("history replacement requires recovery")
	}
}

func TestProjectionEarlyCallAndFinalTurnUpdate(t *testing.T) {
	var m Mirror
	m.Load(proto.Snapshot{Seq: 1})
	m.Apply(ev(2, proto.EventCallDelta, proto.CallDeltaData{Delta: "ec"}))
	m.Apply(ev(3, proto.EventCallDelta, proto.CallDeltaData{CallID: "c", Delta: "ho"}))
	m.Apply(ev(4, proto.EventCallStart, proto.CallStartData{CallID: "c", Command: "echo"}))
	if s := m.Snapshot(); len(s.Live.Calls) != 1 || s.Live.Calls[0].Command != "echo" || !s.StartedCalls["c"] {
		t.Fatalf("early call %+v", s.Live)
	}
	turn := proto.Turn{ID: "a", Role: "assistant", Content: "ok"}
	m.Apply(ev(5, proto.EventTurnAdded, proto.TurnAddedData{Turn: turn}))
	turn.Run = &proto.RunStats{Turns: 2}
	u, ok := m.ApplyUpdate(ev(6, proto.EventTurnUpdated, proto.TurnAddedData{Turn: turn}))
	if !ok || len(u.Values.Turns) != 1 || m.Snapshot().Turns[0].Run.Turns != 2 {
		t.Fatal("final stats lost")
	}
}

func TestProjectionRejectsRetiredBootAndOwnsNestedData(t *testing.T) {
	var m Mirror
	snap := proto.Snapshot{Seq: 10, Version: proto.SessionVersion{BootID: "old", Revision: 10}, Ask: &proto.AskForm{Questions: []proto.AskQuestion{{Options: []string{"one"}}}}}
	m.Load(snap)
	snap.Ask.Questions[0].Options[0] = "changed"
	if m.Snapshot().Ask.Questions[0].Options[0] != "one" {
		t.Fatal("nested snapshot alias")
	}
	m.Load(proto.Snapshot{Seq: 1, Version: proto.SessionVersion{BootID: "new", Revision: 1}})
	if _, ok := m.LoadUpdate(snap); ok {
		t.Fatal("retired boot accepted")
	}
}

func BenchmarkProjectionFragments(b *testing.B) {
	for _, n := range []int{1, 200} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			turns := make([]proto.Turn, 2000)
			for i := range turns {
				turns[i] = proto.Turn{ID: strconv.Itoa(i), Content: "history"}
			}
			var m Mirror
			m.Load(proto.Snapshot{Turns: turns})
			events := make([]proto.Event, n)
			for i := range events {
				events[i] = ev(0, proto.EventTextDelta, proto.TextDeltaData{Text: "x"})
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
