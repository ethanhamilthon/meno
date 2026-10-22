package tui

import (
	"context"
	"meno/internal/api"
	"meno/internal/client"
	"sync"
)

type sessionRefreshed struct {
	sid  string
	snap api.Snapshot
	err  error
}
type wireEv struct {
	sid string
	ev  api.Event
}
type snapshotEv struct {
	sid  string
	snap api.Snapshot
}

func (a *App) testMirror(rt *Runtime) *client.Mirror {
	// The test fixture can intentionally seed UI-only state between events.
	m := &client.Mirror{}
	sn := api.Snapshot{Session: api.SessionItem{ID: rt.ID(), Name: rt.Session.Title}, Running: rt.Running, RequestID: rt.RequestID, Live: api.LiveState{Text: rt.LiveText, Thinking: rt.LiveThink}, Queued: rt.Queued}
	for _, c := range rt.LiveCalls {
		sn.Live.Calls = append(sn.Live.Calls, api.CallState{CallID: c.ID, Command: c.Code})
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
func (a *App) applyWire(rt *Runtime, e api.Event) {
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
