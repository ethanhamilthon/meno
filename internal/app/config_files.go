package app

import (
	"fmt"
	"os"
	"reflect"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/ui"
)

// loadThemeFile registers the themes of theme.json. It must run before the
// saved theme is applied, which may be one of them.
func (a *App) loadThemeFile() {
	raw, warns := config.ReadObjects(a.paths.ThemeFile())
	themes, more := ui.ThemesFromJSON(raw)
	ui.SetExtraThemes(themes)
	a.warnings = append(a.warnings, append(warns, more...)...)
}

func (a *App) loadAll() error {
	var err error
	if a.st.Providers, err = a.store.LoadProviders(); err != nil {
		return err
	}
	if a.st.Skills, err = a.store.LoadSkills(); err != nil {
		return err
	}
	if a.st.Servers, err = a.store.LoadServers(); err != nil {
		return err
	}
	a.mergeOverlays()
	a.initOAuth()
	return nil
}

// mergeOverlays adds provider.json and mcp.json to the saved providers and
// servers. Their entries are read only and never saved.
func (a *App) mergeOverlays() {
	files, warns := config.LoadProviderFile(a.paths.ProviderFile(), os.Getenv)
	var more []string
	a.st.Providers, more = config.MergeProviders(a.st.Providers, files)
	warns = append(warns, more...)
	servers, w := config.LoadMCPFile(a.paths.MCPFile())
	warns = append(warns, w...)
	a.st.Servers, more = config.MergeServers(a.st.Servers, servers)
	a.warnings = append(a.warnings, append(warns, more...)...)
}

// reportWarnings shows what was wrong in the overlay files, once.
func (a *App) reportWarnings() {
	for _, w := range a.warnings {
		a.notifyErr(w)
	}
	a.warnings = nil
}

// fileTag is the second line of a list row for an entry read from a file.
func fileTag(fromFile bool, name string) string {
	if !fromFile {
		return ""
	}
	return "from " + name + " · read only"
}

// fileEntryNotice explains why an entry read from a file cannot be changed here.
func (a *App) fileEntryNotice(name string) {
	a.notifyErr("defined in " + name + " and read only: edit the file and restart meno")
}

// guardFileServers refuses a change that would alter or remove a server read
// from mcp.json.
func guardFileServers(old, next []mcp.Server) error {
	for _, o := range old {
		if !o.FromFile() {
			continue
		}
		kept := false
		for _, n := range next {
			if reflect.DeepEqual(n, o) {
				kept = true
			}
		}
		if !kept {
			return fmt.Errorf("%s is defined in %s and is read only: edit the file and restart meno", o.Name, config.MCPFileName)
		}
	}
	return nil
}

// startProviderRuns starts the run command of every provider that has one. It
// does not wait: a request that arrives before the proxy listens is retried.
func (a *App) startProviderRuns() {
	for _, p := range a.st.Providers {
		a.syncProviderRun(p)
	}
}

// syncProviderRun makes the daemon of p match its run command: started when
// set, replaced when changed, stopped when empty.
func (a *App) syncProviderRun(p llm.Provider) {
	if a.daemons == nil {
		return
	}
	if p.Disabled {
		a.daemons.Stop(p.Name)
		return
	}
	if p.Managed() && a.daemons.RunningCommand(p.Name) != p.Run {
		if err := a.oauth.WriteConfig(); err != nil { // a fresh config before every start
			a.notifyErr("cliproxyapi: " + err.Error())
			return
		}
	}
	if err := a.daemons.Start(p.Name, p.Run, a.paths.Cwd, os.Environ()); err != nil {
		a.notifyErr("provider " + p.Name + ": " + err.Error())
	}
}

// providerRetry is the retry policy for requests to p. A provider with a run
// command may still be starting at the first request, so it gets more attempts
// (about a minute and a half) than the default.
func (a *App) providerRetry(p llm.Provider) agent.RetryPolicy {
	r := a.retry
	if p.Run != "" && r.Attempts == 0 {
		r.Attempts = 9
	}
	return r
}
