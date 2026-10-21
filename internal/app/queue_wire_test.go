package app

import (
	"encoding/json"
	"reflect"
	"testing"

	"meno/internal/proto"
	"meno/internal/ui"
)

func queueTestApp() (*App, *Runtime) {
	rt := &Runtime{Session: &sessView{id: "session", Title: "session"}, Running: true, Mode: ui.ModeInput}
	a := &App{st: &State{Runtime: rt}, rts: map[string]*Runtime{rt.ID(): rt}}
	return a, rt
}

func injectedEvent(t *testing.T, queued []string) proto.Event {
	t.Helper()
	raw, err := json.Marshal(proto.InjectedData{Queued: queued})
	if err != nil {
		t.Fatal(err)
	}
	return proto.Event{Type: proto.EventInjected, Data: raw}
}

func TestInjectedReplacesQueuedMessages(t *testing.T) {
	for _, queued := range [][]string{{"remaining"}, {}, nil} {
		a, rt := queueTestApp()
		rt.Queued = []string{"consumed", "remaining"}
		rt.LiveText, rt.LiveThink = "old text", "old thought"
		rt.LiveCalls = []liveCall{{ID: "old"}}
		a.applyWire(rt, injectedEvent(t, queued))
		if len(rt.Queued) != len(queued) || len(queued) > 0 && !reflect.DeepEqual(rt.Queued, queued) {
			t.Fatalf("queue %q, want %q", rt.Queued, queued)
		}
		if rt.LiveText != "old text" || rt.LiveThink != "old thought" || len(rt.LiveCalls) != 1 {
			t.Fatalf("queue notification discarded live state: %+v", rt)
		}
		if len(queued) == 0 && a.queueHeight() != 0 {
			t.Fatal("the queue panel remains visible after consumption")
		}
	}
}

func TestSteeringQueueUsesEventsNotSendAcknowledgments(t *testing.T) {
	for _, ackFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "ack-before-injection", false: "ack-after-injection"}[ackFirst], func(t *testing.T) {
			a, rt := queueTestApp()
			// Keep the save pending: no HTTP request is needed for this state test.
			a.saves.busy = true
			a.send(msgDraft{Text: "steer"})
			if len(rt.Queued) != 0 || rt.sending != 1 {
				t.Fatal("sending must not add an unconfirmed queue entry")
			}
			ack := sendResult{sid: rt.ID(), msg: msgDraft{Text: "steer"}, resp: proto.MessageResponse{Queued: true}}
			if ackFirst {
				a.onSendResult(ack)
			}
			a.applyWire(rt, injectedEvent(t, []string{"steer"}))
			a.applyWire(rt, injectedEvent(t, []string{"steer"}))
			if !reflect.DeepEqual(rt.Queued, []string{"steer"}) {
				t.Fatalf("enqueue event duplicated the message: %q", rt.Queued)
			}
			a.applyWire(rt, injectedEvent(t, []string{}))
			if !ackFirst {
				a.onSendResult(ack)
			}
			if len(rt.Queued) != 0 || rt.sending != 0 || a.queueHeight() != 0 {
				t.Fatalf("a consumed message returned to the queue: %q", rt.Queued)
			}
		})
	}
}

func TestInjectedUpdatesBackgroundSessionQueue(t *testing.T) {
	a, current := queueTestApp()
	current.Queued, current.LiveText = []string{"current"}, "current text"
	background := &Runtime{Session: &sessView{id: "background"}, Queued: []string{"consumed"}, Running: true}
	a.rts[background.ID()] = background
	a.onWire(wireEv{sid: background.ID(), ev: injectedEvent(t, []string{})})
	if len(background.Queued) != 0 {
		t.Fatal("the background session kept its consumed message")
	}
	if a.st.Runtime != current || !reflect.DeepEqual(current.Queued, []string{"current"}) || current.LiveText != "current text" {
		t.Fatal("the background event changed the current session")
	}
}
