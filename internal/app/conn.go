package app

import (
	"context"
	"time"

	"meno/internal/client"
	"meno/internal/proto"
	"meno/internal/safe"
)

// wireEv is one event of a session stream, delivered to the UI goroutine.
type wireEv struct {
	sid string
	ev  proto.Event
}

// wireLost tells the UI that a session stream broke and could not be resumed.
type wireLost struct {
	sid string
	err error
}

// snapshotEv delivers a fresh snapshot (after attach or resync).
type snapshotEv struct {
	sid  string
	snap proto.Snapshot
}

// rpc runs fn against the daemon with a time limit. It blocks, so call it off
// the UI goroutine, or only for quick calls.
func (a *App) rpc(fn func(ctx context.Context, c *client.Client) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return fn(ctx, a.cl)
}

// rpcCtx is rpc under a caller's context (a login that can be cancelled).
func (a *App) rpcCtx(ctx context.Context, fn func(ctx context.Context, c *client.Client) error) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return fn(ctx, a.cl)
}

// spawn runs fn off the UI goroutine and keeps Close waiting for it.
func (a *App) spawn(fn func()) {
	a.running.Add(1)
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.running.Done()
		fn()
	}()
}

// watch streams the events of a session into the UI until stop is called. It
// takes a snapshot first, so nothing is missed between the snapshot and the
// stream, and again whenever the daemon asks for a resync.
func (a *App) watch(sid string) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	a.watchers.Add(1)
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.watchers.Done()
		mirror := &client.Mirror{}
		err := a.cl.AttachWith(ctx, sid, mirror, func(sn proto.Snapshot) { a.postEvent(snapshotEv{sid, sn}) }, func(e proto.Event) error {
			a.postEvent(wireEv{sid, e})
			return nil
		})
		if err != nil && ctx.Err() == nil {
			a.postEvent(wireLost{sid, err})
		}
	}()
	return cancel
}
