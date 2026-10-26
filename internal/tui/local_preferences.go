package tui

import (
	"meno/internal/config"
	"meno/internal/render"
)

// localKeys are the settings a window may override for itself: how it looks
// and how it signals. Everything else is shared by all windows.
var localKeys = map[string]bool{
	"notify": true, "notify_sound": true, "notify_volume": true, "notify_command": true,
}

// noteSettingEdit records where an edit of key belongs. With "this window"
// selected the new value is an override that never leaves this window; an edit
// in the shared scope replaces the override, so the shared value wins again.
func (a *App) noteSettingEdit(key string) {
	if a.st.LocalScope && localKeys[key] {
		if a.win.local == nil {
			a.win.local = map[string]string{}
		}
		a.win.local[key] = a.st.Settings.Get(key)
		return
	}
	delete(a.win.local, key)
}

// sharedSettings is the current settings without this window's overrides: what
// would be sent to the daemon.
func (a *App) sharedSettings() config.Settings {
	s := a.st.Settings
	for k := range a.win.local {
		_ = s.Set(k, a.cfgBase.Get(k))
	}
	return s
}

// applyOverrides puts this window's overrides back over settings that were
// just read from the daemon.
func (a *App) applyOverrides() {
	for k, v := range a.win.local {
		_ = a.st.Settings.Set(k, v)
	}
}

// resetLocal drops every override: the window follows the shared defaults again.
func (a *App) resetLocal() {
	if len(a.win.local) == 0 {
		return
	}
	for k := range a.win.local {
		_ = a.st.Settings.Set(k, a.cfgBase.Get(k))
	}
	a.win.local = nil
}

// windowState is what belongs to this window alone and never reaches the daemon.
type windowState struct {
	local         map[string]string // overrides of look and notification settings
	statusMidX    int               // column where the middle of the status bar starts
	statusMidSegs []render.Seg      // what is drawn there, for clicks
}
