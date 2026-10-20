package client

import (
	"encoding/json"
	"meno/internal/proto"
	"testing"
)

func TestMirrorConsumesSteeringQueue(t *testing.T) {
	m := &Mirror{}
	m.Load(proto.Snapshot{Seq: 1})
	for i, queue := range [][]string{{"one"}, {"one", "two"}, {"two"}, {}} {
		data, err := json.Marshal(proto.InjectedData{Queued: queue})
		if err != nil {
			t.Fatal(err)
		}
		m.Apply(proto.Event{Seq: uint64(i + 2), Type: proto.EventInjected, Data: data})
		got := m.Snapshot().Queued
		if len(got) != len(queue) {
			t.Fatalf("queue after event %d: %v, want %v", i, got, queue)
		}
	}
}
