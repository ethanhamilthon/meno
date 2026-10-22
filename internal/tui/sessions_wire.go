package tui

import (
	"context"
	"time"

	"meno/internal/api"
	"meno/internal/client"
)

// createSession asks the daemon for an empty session in cwd.
func (a *App) createSession(cwd string) (*sessView, error) {
	var sn api.Snapshot
	err := a.rpc(func(ctx context.Context, c *client.Client) error {
		r, err := c.CreateSession(ctx, api.SessionCreateRequest{Cwd: cwd, ClientID: newClientID()})
		if err != nil {
			return err
		}
		sn, err = c.Snapshot(ctx, r.Session.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	s := &sessView{}
	s.load(sn)
	return s, nil
}

// newSession starts an empty session in the directory of the current one.
func (a *App) newSession() {
	cwd := a.st.Cwd
	s, err := a.createSession(cwd)
	if err != nil {
		a.notifyErr("new session failed: " + err.Error())
		return
	}
	rt := a.newRuntime(s, cwd)
	rt.watch = a.watch(s.ID())
	a.switchTo(rt)
}

// openSession puts a session on screen: one already followed as it is, any
// other attached from the daemon. Running turns are left alone.
func (a *App) openSession(id string) {
	a.closeOverlay()
	if rt := a.rts[id]; rt != nil {
		a.switchTo(rt)
		return
	}
	var sn api.Snapshot
	err := a.rpc(func(ctx context.Context, c *client.Client) error {
		var e error
		sn, e = c.Snapshot(ctx, id)
		return e
	})
	if err != nil {
		a.notifyErr("open failed: " + err.Error())
		return
	}
	s := &sessView{}
	s.load(sn)
	cwd := sn.Cwd
	if cwd == "" || !dirExists(cwd) {
		a.notifyErr("directory " + cwd + " no longer exists · using " + a.paths.Cwd)
		cwd = a.paths.Cwd
	}
	rt := a.newRuntime(s, cwd)
	rt.watch = a.watch(id)

	a.switchTo(rt)
	a.notify("session opened")
}

// patchSession changes metadata of the current session in the daemon.
func (a *App) patchSession(p api.PatchSessionRequest, done func(error)) {
	sid := a.st.Session.ID()
	a.pushSave(func() {
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			_, e := c.PatchSession(ctx, sid, p)
			return e
		})
		a.postEvent(uiCall{fn: func() { done(err) }, done: make(chan struct{})})
	})
}

// setSelection gives the current session a model. It also becomes the default
// for sessions started later. A turn that is running keeps the model it
// started with.
func (a *App) setSelection(provider, model, effort string) {
	if s := a.st.Session; s != nil {
		s.Provider, s.Model, s.Effort = provider, model, effort
		a.patchSession(api.PatchSessionRequest{Provider: &provider, Model: &model, Effort: &effort}, func(err error) {
			if err != nil {
				a.notifyErr("model: " + err.Error())
			}
		})
	}
	a.st.Settings.Provider, a.st.Settings.Model, a.st.Settings.Effort = provider, model, effort
	a.persistSettings()
}

// setCwd moves a session to dir.
func (a *App) setCwd(rt *Runtime, dir string) {
	rt.PrevCwd, rt.Cwd = rt.Cwd, dir
	rt.Session.Cwd = dir
	sid := rt.ID()
	a.pushSave(func() {
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			_, e := c.PatchSession(ctx, sid, api.PatchSessionRequest{Cwd: &dir})
			return e
		})
		if err != nil {
			a.postEvent(errNotice{text: "cd: " + err.Error()})
		}
	})
}

// renameSession handles "/rename <name>".
func (a *App) renameSession(arg string) {
	if arg == "" {
		a.notify("usage: /rename <name> · now: " + a.st.Session.DisplayTitle())
		return
	}
	a.st.Session.setTitle(arg)
	a.patchSession(api.PatchSessionRequest{Name: &arg}, func(err error) {
		if err != nil {
			a.notifyErr("rename failed: " + err.Error())
			return
		}
		a.notify("session renamed")
	})
}

// compact asks the daemon to compact the context of the current session.
func (a *App) compact() {
	rt := a.st.Runtime
	if rt.Running {
		a.notifyErr("agent is running · /stop it first (or ctrl+c)")
		return
	}
	sid := rt.ID()
	rt.Running, rt.Phase = true, "compacting"
	a.spawn(func() {
		var resp api.CompactResponse
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			var e error
			resp, e = c.Compact(cctx, sid, api.CompactRequest{ClientID: newClientID()})
			return e
		})
		a.postEvent(compactDone{sid, resp, err})
	})
}

func (a *App) onCompactDone(m compactDone) {
	rt := a.rts[m.sid]
	if rt == nil {
		return
	}
	rt.Running, rt.Phase = false, ""
	if m.err != nil {
		a.noticeFor(rt, "compact failed: "+m.err.Error(), true, false)
		return
	}
	a.noticeFor(rt, "context compacted · "+itoa(m.resp.Removed)+" turns folded into a summary", false, false)
}

// openSessions lists the sessions of the daemon in the background.
func (a *App) openSessions() {
	a.openOverlay(OverSessions)
	a.invalidateGlobal()
}
