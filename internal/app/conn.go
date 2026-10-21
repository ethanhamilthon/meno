package app

import (
	"context"
	"fmt"
	"time"

	"meno/internal/client"
	"meno/internal/safe"
)

// projectionEv is an immutable accepted update for one watcher generation.
type projectionEv struct {
	sid        string
	generation uint64
	update     client.ProjectionUpdate
}
type wireLost struct {
	sid        string
	generation uint64
	err        error
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
	a.watchGeneration++
	generation := a.watchGeneration
	a.rts[sid].generation = generation
	a.watchers.Add(1)
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.watchers.Done()
		err := a.cl.Follow(ctx, sid, func(u client.ProjectionUpdate) error {
			a.postEvent(projectionEv{sid: sid, generation: generation, update: u})
			return nil
		})
		if ctx.Err() == nil {
			if err == nil {
				err = fmt.Errorf("daemon stream closed")
			}
			a.postEvent(wireLost{sid: sid, generation: generation, err: err})
		}
	}()
	return cancel
}
