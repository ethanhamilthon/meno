package client

import (
	"encoding/json"
	"meno/internal/api"
	"testing"
)

func TestMirrorConsumesSteeringQueue(t *testing.T) {
	m := &Mirror{}
	m.Load(api.Snapshot{Seq: 1})
	for i, queue := range [][]string{{"one"}, {"one", "two"}, {"two"}, {}} {
		data, err := json.Marshal(api.InjectedData{Queued: queue})
		if err != nil {
			t.Fatal(err)
		}
		m.Apply(api.Event{Seq: uint64(i + 2), Type: api.EventInjected, Data: data})
		got := m.Snapshot().Queued
		if len(got) != len(queue) {
			t.Fatalf("queue after event %d: %v, want %v", i, got, queue)
		}
	}
}
