package app

import (
	"context"
	"time"

	"meno/internal/client"
	"meno/internal/proto"
)

// createSession asks the daemon for an empty session in cwd.
func (a *App) createSession(cwd string) (*sessView, error) {
	var sn proto.Snapshot
	err := a.rpc(func(ctx context.Context, c *client.Client) error {
		r, err := c.CreateSession(ctx, proto.SessionCreateRequest{Cwd: cwd, ClientID: newClientID()})
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
	rt.Saved = true
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
	var sn proto.Snapshot
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
	a.onSnapshot(snapshotEv{id, sn})
	a.switchTo(rt)
	a.notify("session opened")
}

// refreshSession re-reads a session after the daemon said something about it
// changed (title, directory, tasks, agents).
func (a *App) refreshSession(sid string) {
	a.spawn(func() {
		var sn proto.Snapshot
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			var e error
			sn, e = c.Snapshot(ctx, sid)
			return e
		})
		a.postEvent(sessionRefreshed{sid, sn, err})
	})
}

// onSessionRefreshed applies the parts of a snapshot that are not live: the
// title, the directory, the model and the background work.
func (a *App) onSessionRefreshed(m sessionRefreshed) {
	rt := a.rts[m.sid]
	if rt == nil || m.err != nil {
		return
	}
	sn := m.snap
	rt.Session.Title = sn.Session.Name
	if rt.Session.Title == "untitled" {
		rt.Session.Title = ""
	}
	rt.Session.Provider, rt.Session.Model, rt.Session.Effort = sn.Provider, sn.Model, sn.Effort
	rt.Session.Modes = append([]string(nil), sn.Modes...)
	if sn.Cwd != "" {
		rt.Cwd = sn.Cwd
	}
	a.setTasks(rt, sn.Tasks)
	a.setKids(rt, sn.Agents)
}

// patchSession changes metadata of the current session in the daemon.
func (a *App) patchSession(p proto.PatchSessionRequest, done func(error)) {
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
		a.patchSession(proto.PatchSessionRequest{Provider: &provider, Model: &model, Effort: &effort}, func(err error) {
			if err != nil {
				a.notifyErr("model: " + err.Error())
			}
		})
	}
	a.st.Settings.Provider, a.st.Settings.Model, a.st.Settings.Effort = provider, model, effort
	a.persistSettings()
}

// setModes replaces the modes of the current session. Unknown names are dropped.
func (a *App) setModes(names []string) {
	var keep []string
	for _, n := range names {
		if _, ok := findMode(n); ok && !hasName(keep, n) {
			keep = append(keep, n)
		}
	}
	a.st.Session.Modes = keep
	a.patchSession(proto.PatchSessionRequest{Modes: &keep}, func(err error) {
		if err != nil {
			a.notifyErr("modes: " + err.Error())
		}
	})
}

// setCwd moves a session to dir.
func (a *App) setCwd(rt *Runtime, dir string) {
	rt.PrevCwd, rt.Cwd = rt.Cwd, dir
	rt.Session.Cwd = dir
	sid := rt.ID()
	a.pushSave(func() {
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			_, e := c.PatchSession(ctx, sid, proto.PatchSessionRequest{Cwd: &dir})
			return e
		})
		if err != nil {
			a.postEvent(errNotice{text: "cd: " + err.Error()})
		}
	})
}

// renameSession handles ":rename <name>".
func (a *App) renameSession(arg string) {
	if arg == "" {
		a.notify("usage: :rename <name> · now: " + a.st.Session.DisplayTitle())
		return
	}
	a.st.Session.setTitle(arg)
	a.patchSession(proto.PatchSessionRequest{Name: &arg}, func(err error) {
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
		a.notifyErr("agent is running · :stop it first (or ctrl+c)")
		return
	}
	sid := rt.ID()
	rt.Running, rt.Phase = true, "compacting"
	a.spawn(func() {
		var resp proto.CompactResponse
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			var e error
			resp, e = c.Compact(cctx, sid, proto.CompactRequest{ClientID: newClientID()})
			return e
		})
		a.postEvent(compactDone{sid, resp, err})
	})
}

func (a *App) onCompactDone(m compactDone) {
	a.inRT(m.sid, func() {
		a.st.Running, a.st.Phase = false, ""
		if m.err != nil {
			a.notifyErr("compact failed: " + m.err.Error())
			return
		}
		a.notify("context compacted · " + itoa(m.resp.Removed) + " turns folded into a summary")
		a.refreshSession(m.sid)
	})
}

// openSessions lists the sessions of the daemon in the background.
func (a *App) openSessions() {
	a.openOverlay(OverSessions)
	a.spawn(func() {
		var list proto.SessionsListResponse
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			var e error
			list, e = c.ListSessions(ctx, proto.SessionsListRequest{Limit: maxSessionRows})
			return e
		})
		a.postEvent(sessionsLoaded{list.Sessions, err})
	})
}

func (a *App) onSessionsLoaded(m sessionsLoaded) {
	if m.err != nil {
		a.notifyErr("sessions failed: " + m.err.Error())
		return
	}
	a.dbRows = m.list
}
