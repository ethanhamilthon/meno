package tui

import (
	"testing"

	"meno/internal/config"
)

func openSettingsAt(t *testing.T, a *App, key string) {
	t.Helper()
	a.openOverlay(OverSettings)
	a.st.Sel = settingIdx(t, key)
}

// A window can keep its own theme and notification mode; the shared value on
// the daemon is untouched and the override survives a refresh of the settings.
func TestLocalOverrideStaysInThisWindow(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	shared := a.st.Settings.Notify
	openSettingsAt(t, a, "notify")
	a.onKey(key('s')) // this window only
	if !a.st.LocalScope {
		t.Fatal("scope did not switch")
	}
	a.onKey(key(' ')) // next notify mode
	changed := a.st.Settings.Notify
	if changed == shared {
		t.Fatal("setting did not change")
	}
	if got := a.sharedSettings().Notify; got != shared {
		t.Fatalf("override would be sent to the daemon: %q", got)
	}
	if fields, _ := settingsDelta(a.cfgBase, a.sharedSettings()); len(fields) != 0 {
		t.Fatalf("local edit produced a patch: %v", fields)
	}
	// The daemon reports its settings again: the override stays on top.
	m := settingsLoaded{settings: a.cfgBase, providers: a.st.Providers, servers: a.st.Servers, skills: a.st.Skills}
	a.applyConfig(m)
	if a.st.Settings.Notify != changed {
		t.Fatalf("refresh lost the override: %q", a.st.Settings.Notify)
	}
}

// Reset returns the window to the shared defaults, theme included.
func TestResetLocalReturnsToSharedDefaults(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	shared := a.st.Settings
	openSettingsAt(t, a, "notify")
	a.onKey(key('s'))
	a.onKey(key(' '))
	a.onKey(key('r'))
	if a.st.Settings.Notify != shared.Notify || len(a.win.local) != 0 {
		t.Fatalf("notify %q, overrides %v", a.st.Settings.Notify, a.win.local)
	}
}

// In the shared scope an edit goes to the daemon and clears an old override of
// the same setting, so the shared value is what the window uses.
func TestSharedEditReplacesOverride(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	initial := a.st.Settings.Notify
	openSettingsAt(t, a, "notify")
	a.onKey(key('s'))
	a.onKey(key(' '))
	a.onKey(key('s')) // back to shared
	a.onKey(key(' '))
	if _, ok := a.win.local["notify"]; ok {
		t.Fatal("the override outlived a shared edit")
	}
	if a.cfgBase.Notify != a.st.Settings.Notify || a.cfgBase.Notify == initial {
		t.Fatalf("a shared edit must be sent: base %q, window %q, initial %q", a.cfgBase.Notify, a.st.Settings.Notify, initial)
	}
}

// Settings that are not about looks or signals are always shared.
func TestOnlyLooksAndSignalsCanBeLocal(t *testing.T) {
	for _, k := range []string{"images", "bash_code", "compact_percent", "thinking"} {
		if localKeys[k] {
			t.Errorf("%s must stay shared", k)
		}
	}
	_ = config.Settings{}
}
