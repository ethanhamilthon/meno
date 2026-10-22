package tui

import (
	"context"
	"encoding/json"
	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/safe"
	"time"
)

type globalWatchState struct {
	stop                  context.CancelFunc
	generation            uint64
	pending, dirty, stale bool
}
type globalRetry struct{ generation uint64 }

type globalWatchEvent struct{ ev api.Event }
type globalSessionsLoaded struct {
	generation uint64
	rows       []api.SessionItem
	tasks      int
	err        error
}

func (a *App) startGlobalWatch() {
	if a.cl == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.globalWatch.stop = cancel
	a.watchers.Add(1)
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.watchers.Done()
		_ = a.cl.WatchGlobal(ctx, func(ev api.Event) error {
			a.postEvent(globalWatchEvent{ev: ev})
			return nil
		})
	}()
}

func (a *App) onGlobalWatch(m globalWatchEvent) {
	switch m.ev.Type {
	case client.GlobalUnavailable, api.EventBye:
		a.globalWatch.stale = true
		a.globalWatch.generation++
	case api.EventHello:
		a.globalWatch.generation++
		a.invalidateGlobal()
	case api.EventGlobalStatus, api.EventSessionCreated, api.EventSessionDeleted:
		a.invalidateGlobal()
	case "settings_changed", "providers_changed", api.EventMCPChanged:
		a.refreshConfig()
	}
}

func (a *App) invalidateGlobal() {
	a.globalWatch.dirty = true
	a.refreshGlobalState()
}

// refreshGlobalState is event-driven: at most one request and one trailing
// refresh, even when many invalidations race with a slow hydration.
func (a *App) refreshGlobalState() {
	if a.cl == nil || a.globalWatch.pending || !a.globalWatch.dirty || a.closing {
		return
	}
	a.globalWatch.dirty = false
	a.globalWatch.pending = true
	generation := a.globalWatch.generation
	a.spawn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		r, err := a.cl.Resource(ctx, "GET", "/v1/session-state", "", api.ResourceRequest{})
		var state struct {
			Sessions []api.SessionItem `json:"sessions"`
			Tasks    int               `json:"tasks"`
		}
		if err == nil {
			err = json.Unmarshal(r.Data, &state)
		}
		a.postEvent(globalSessionsLoaded{generation: generation, rows: state.Sessions, tasks: state.Tasks, err: err})
	})
}

func (a *App) onGlobalSessions(m globalSessionsLoaded) {
	a.globalWatch.pending = false
	if m.generation == a.globalWatch.generation {
		a.globalWatch.stale = m.err != nil
		if m.err != nil && a.st.Over == OverSessions {
			a.notifyErr("sessions failed: " + m.err.Error())
		}
		if m.err == nil {
			a.globalSessions, a.globalTasks = m.rows, m.tasks
			for _, row := range m.rows {
				if rt := a.rts[row.ID]; rt != nil && rt.Session != nil {
					rt.Session.Completed, rt.Session.ReadThrough = row.Completed, row.ReadThrough
					rt.Unseen = row.Completed > row.ReadThrough
				}
			}
		}
	}
	if m.err != nil && m.generation == a.globalWatch.generation && !a.closing {
		generation := m.generation
		a.spawn(func() {
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				a.postEvent(globalRetry{generation: generation})
			case <-a.done:
			}
		})
	}
	a.refreshGlobalState()
}
