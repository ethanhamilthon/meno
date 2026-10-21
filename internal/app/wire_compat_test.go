package app

import (
	"context"
	"meno/internal/client"
	"meno/internal/proto"
	"sync"
)

type sessionRefreshed struct {
	sid  string
	snap proto.Snapshot
	err  error
}
type wireEv struct {
	sid string
	ev  proto.Event
}
type snapshotEv struct {
	sid  string
	snap proto.Snapshot
}

func (a *App) testMirror(rt *Runtime) *client.Mirror {
	// The test fixture can intentionally seed UI-only state between events.
	m := &client.Mirror{}
	sn := proto.Snapshot{Session: proto.SessionItem{ID: rt.ID(), Name: rt.Session.Title}, Running: rt.Running, RequestID: rt.RequestID, Live: proto.LiveState{Text: rt.LiveText, Thinking: rt.LiveThink}, Queued: rt.Queued}
	for _, c := range rt.LiveCalls {
		sn.Live.Calls = append(sn.Live.Calls, proto.CallState{CallID: c.ID, Command: c.Code})
	}
	m.Load(sn)
	return m
}

var fixtureMirrors sync.Map

func fixtureMirror(rt *Runtime) *client.Mirror {
	m, _ := fixtureMirrors.Load(rt)
	if m == nil {
		return nil
	}
	return m.(*client.Mirror)
}
func (a *App) applyWire(rt *Runtime, e proto.Event) {
	m := fixtureMirror(rt)
	if m == nil {
		m = a.testMirror(rt)
		fixtureMirrors.Store(rt, m)
	}
	if u, ok := m.ApplyUpdate(e); ok {
		a.applyProjection(rt, u)
	}
}
func (a *App) onWire(m wireEv) {
	if rt := a.rts[m.sid]; rt != nil {
		a.applyWire(rt, m.ev)
	}
}
func (a *App) onSnapshot(m snapshotEv) {
	rt := a.rts[m.sid]
	if rt == nil {
		return
	}
	mirror := fixtureMirror(rt)
	if mirror == nil {
		mirror = &client.Mirror{}
		fixtureMirrors.Store(rt, mirror)
	}
	if u, ok := mirror.LoadUpdate(m.snap); ok {
		a.applyProjection(rt, u)
	}
}
func (a *App) refreshSession(sid string) {
	sn, err := a.cl.Snapshot(context.Background(), sid)
	if err == nil {
		a.onSnapshot(snapshotEv{sid, sn})
	}
}
func (a *App) onSessionRefreshed(m sessionRefreshed) {
	if m.err == nil {
		a.onSnapshot(snapshotEv{m.sid, m.snap})
	}
}
