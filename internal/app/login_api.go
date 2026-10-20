package app

import (
	"strings"

	"meno/internal/llm"
)

// The API-key side of :login: providers the user adds by hand. They are saved
// in meno.db and can be edited, switched off and removed.

// usable reports whether a provider can serve a request: it is switched on,
// valid, and has a key (or runs locally, or comes from the OAuth accounts).
func usable(p llm.Provider) bool {
	if p.Disabled {
		return false
	}
	if p.Managed() {
		return true
	}
	q := p // an older or hand-written protocol name still works at run time
	if q.Protocol != llm.ProtoResponses {
		q.Protocol = llm.ProtoChat
	}
	if llm.Validate(q) != nil {
		return false
	}
	u := strings.ToLower(p.BaseURL)
	local := strings.Contains(u, "://127.0.0.1") || strings.Contains(u, "://localhost") || strings.Contains(u, "://[::1]")
	return p.APIKey != "" || p.Run != "" || local
}

// needProvider reports that nothing could answer a request yet.
func (a *App) needProvider() bool {
	for _, p := range a.st.Providers {
		if usable(p) {
			return false
		}
	}
	return true
}

// enforceProvider opens the login list, which cannot be closed, when meno
// starts without a working provider.
func (a *App) enforceProvider() {
	if a.needProvider() {
		a.st.Locked = true
		a.openOverlay(OverLogin)
		a.st.Sel = firstAddRow(a.loginRows())
	}
}

// afterProvidersChanged lifts the lock as soon as a provider works.
func (a *App) afterProvidersChanged() {
	if a.st.Locked && !a.needProvider() {
		a.st.Locked = false
		if a.st.Over == OverLogin || a.st.Over == OverLoginFlow {
			a.closeOverlay()
		}
		a.notify("ready · choose a model with :model")
	}
	if !a.st.Locked && a.needProvider() && a.st.Over == OverNone {
		a.enforceProvider() // the last provider went away
	}
}

// newAPIDraft is the start of a provider added by hand.
func (a *App) newAPIDraft(proto string) llm.Provider {
	p := llm.Provider{Protocol: proto}
	if proto == llm.ProtoResponses {
		p.Name, p.BaseURL = "openai", "https://api.openai.com/v1"
	}
	for n := 2; p.Name != "" && a.providerNamed(p.Name) >= 0; n++ {
		p.Name = strings.TrimRight(p.Name, "0123456789-") + "-" + itoa(n)
	}
	return p
}

// providerNamed is the index of the provider with this name, or -1.
func (a *App) providerNamed(name string) int {
	for i, p := range a.st.Providers {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// toggleProvider switches an API provider off or on.
func (a *App) toggleProvider(i int) {
	if i < 0 || i >= len(a.st.Providers) {
		return
	}
	p := &a.st.Providers[i]
	if p.FromFile() {
		a.fileEntryNotice("provider.json")
		return
	}
	p.Disabled = !p.Disabled
	a.persistProviders()
	a.fetchModels()
	a.notify(p.Name + map[bool]string{true: " is off", false: " is on"}[p.Disabled])
	a.afterProvidersChanged()
}

// deleteProvider removes an API provider and stops its run command.
func (a *App) deleteProvider(i int) {
	if i < 0 || i >= len(a.st.Providers) {
		return
	}
	p := a.st.Providers[i]
	if p.FromFile() {
		a.fileEntryNotice("provider.json")
		return
	}
	a.st.Providers = append(a.st.Providers[:i], a.st.Providers[i+1:]...)
	a.persistProviders()
	a.notify("removed " + p.Name)
	a.afterProvidersChanged()
}
