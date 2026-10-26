package tui

import (
	"meno/internal/client"
	"testing"
	"time"
)

func TestStreamingDeadlineAndImmediateInput(t *testing.T) {
	now := time.Now()
	s := frameSchedule{last: now}
	if s.delay(now.Add(time.Millisecond), false) <= 0 {
		t.Fatal("uncapped delta")
	}
	if s.delay(now.Add(time.Millisecond), true) != 0 {
		t.Fatal("input delayed")
	}
	if s.delay(now.Add(time.Second), false) != 0 {
		t.Fatal("frame starved")
	}
}
func TestStaleAndInactiveFragmentsDoNotDirtyFrame(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	rt := a.st.Runtime
	if a.eventVisible(projectionEv{sid: rt.ID(), generation: rt.generation + 1, update: client.ProjectionUpdate{Changed: client.LiveChanged}}) {
		t.Fatal("stale visible")
	}
	rt2 := a.newRuntime(&sessView{id: "background"}, a.paths.Cwd)
	if a.eventVisible(projectionEv{sid: rt2.ID(), generation: rt2.generation, update: client.ProjectionUpdate{Changed: client.LiveChanged}}) {
		t.Fatal("inactive fragment visible")
	}
	if !a.eventVisible(projectionEv{sid: rt2.ID(), generation: rt2.generation, update: client.ProjectionUpdate{Changed: client.ActivityChanged}}) {
		t.Fatal("background status suppressed")
	}
}

func TestNoOpEventsDoNotBypassStreamDeadline(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	if a.frameUrgent(tick{}) || a.frameUrgent(themeLoaded{}) {
		t.Fatal("no-op event bypassed frame deadline")
	}
	if !a.frameUrgent(key('a')) {
		t.Fatal("input was not immediate")
	}
}
