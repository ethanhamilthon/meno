package app

import (
	"encoding/json"
	"meno/internal/client"
	"meno/internal/proto"
	"testing"
	"time"
)

func timelineEvent(seq uint64, typ string, data any) proto.Event {
	raw, _ := json.Marshal(data)
	return proto.Event{Seq: seq, BootID: "boot", Type: typ, Data: raw}
}

func TestTimelineRejectsStaleSnapshotAndReplayedTurn(t *testing.T) {
	a, rt := queueTestApp()
	snap := proto.Snapshot{Session: proto.SessionItem{ID: rt.ID()}, Seq: 4, Version: proto.SessionVersion{BootID: "boot", Revision: 4}, Running: true}
	a.onSnapshot(snapshotEv{rt.ID(), snap})
	turn := proto.Turn{ID: "a", Role: "assistant", Content: "answer"}
	a.onWire(wireEv{rt.ID(), timelineEvent(5, proto.EventTurnAdded, proto.TurnAddedData{Turn: turn})})
	a.onSnapshot(snapshotEv{rt.ID(), snap})
	a.onSessionRefreshed(sessionRefreshed{sid: rt.ID(), snap: snap})
	a.onWire(wireEv{rt.ID(), timelineEvent(5, proto.EventTurnAdded, proto.TurnAddedData{Turn: turn})})
	if rt.Session.Len() != 1 {
		t.Fatalf("history rolled back or duplicated: %+v", rt.Session.turns)
	}
	rt.Session.add(turn)
	rt.Session.add(proto.Turn{ID: "b", Role: "assistant", Content: "answer"})
	if rt.Session.Len() != 2 {
		t.Fatal("deduplication must use stable identity, not text")
	}
}

func TestTimelineOldRunStatusCannotStopNewRun(t *testing.T) {
	a, rt := queueTestApp()
	rt.RequestID = "new"
	a.applyWire(rt, timelineEvent(1, proto.EventStatus, proto.StatusData{Running: false, RequestID: "old"}))
	a.applyWire(rt, timelineEvent(2, proto.EventDone, proto.DoneData{RequestID: "old"}))
	if !rt.Running {
		t.Fatal("old run stopped current run")
	}
}

func TestTimelineAssistantCommitClearsThinkingButPreservesCalls(t *testing.T) {
	a, rt := queueTestApp()
	rt.LiveThink = "thought"
	rt.LiveCalls = []liveCall{{ID: "parallel", Running: true}}
	a.applyWire(rt, timelineEvent(1, proto.EventTurnAdded, proto.TurnAddedData{Turn: proto.Turn{ID: "1", Role: "assistant"}}))
	if rt.LiveThink != "" || len(rt.LiveCalls) != 1 {
		t.Fatal("empty assistant boundary lost calls or retained thought")
	}
}

func TestProjectionUpdatedAtParsingAndMalformedPreservation(t *testing.T) {
	a, rt := queueTestApp()
	validStr := "2026-10-21T15:04:05.123456789Z"
	validTime, _ := time.Parse(time.RFC3339Nano, validStr)

	u := client.ProjectionUpdate{
		Changed: client.MetadataChanged,
		Values: client.MirrorState{
			Session: proto.SessionItem{ID: rt.ID(), UpdatedAt: validStr},
		},
	}
	a.applyProjection(rt, u)
	if !rt.Session.UpdatedAt.Equal(validTime) {
		t.Fatalf("expected UpdatedAt %v, got %v", validTime, rt.Session.UpdatedAt)
	}

	uBad := client.ProjectionUpdate{
		Changed: client.MetadataChanged,
		Values: client.MirrorState{
			Session: proto.SessionItem{ID: rt.ID(), UpdatedAt: "invalid-timestamp"},
		},
	}
	a.applyProjection(rt, uBad)
	if !rt.Session.UpdatedAt.Equal(validTime) {
		t.Fatalf("malformed timestamp erased valid UpdatedAt: %v", rt.Session.UpdatedAt)
	}
}
