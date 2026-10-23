package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/render"
)

// loadThemeFile registers the themes of theme.json. It must run before the
// saved theme is applied, which may be one of them.
func (a *App) loadThemeFile() {
	raw, warns := config.ReadObjects(a.paths.ThemeFile())
	themes, more := render.ThemesFromJSON(raw)
	render.SetExtraThemes(themes)
	a.warnings = append(a.warnings, append(warns, more...)...)
}

// settingsLoaded carries the daemon's configuration to the UI goroutine.
type settingsLoaded struct {
	seq       uint64 // order of the read: later reads win
	settings  config.Settings
	providers []llm.Provider
	servers   []mcp.Server
	info      map[string]mcpInfo
	warnings  []string // what is wrong with provider.json and mcp.json
	skills    []config.Skill
	err       error
}

// loadAll reads the configuration from the daemon before the first frame.
func (a *App) loadAll() error {
	m := a.fetchConfig()
	if m.err != nil {
		return m.err
	}
	a.applyConfig(m)
	return nil
}

// refreshConfig re-reads the configuration in the background.
func (a *App) refreshConfig() {
	if a.cfgPending {
		a.cfgDirty = true
		return
	}
	a.cfgPending = true
	a.spawn(func() { a.postEvent(a.fetchConfig()) })
}

func (a *App) onSettingsLoaded(m settingsLoaded) {
	a.cfgPending = false
	if a.cfgDirty {
		a.cfgDirty = false
		defer a.refreshConfig()
	}
	if m.err != nil {
		a.notifyErr("config: " + m.err.Error())
		return
	}
	// A reply that left the daemon before my last edit reached it would undo
	// that edit: only the newest read is applied.
	if m.seq < a.cfgSeen {
		return
	}
	a.cfgSeen = m.seq
	a.applyConfig(m)
	a.showFileWarnings(m.warnings)
	a.fetchModels()
	a.afterProvidersChanged()
}

func (a *App) applyConfig(m settingsLoaded) {
	a.st.Settings, a.st.Providers, a.st.Servers, a.st.Skills = m.settings, m.providers, m.servers, m.skills
	a.st.ServerInfo = m.info
	a.cfgBase = m.settings
	a.applyOverrides()
	a.cfgProv = map[string]llm.Provider{}
	for _, p := range m.providers {
		a.cfgProv[p.Name] = p
	}
}

func (a *App) fetchConfig() settingsLoaded {
	out := settingsLoaded{seq: a.cfgSeq.Add(1)}
	out.err = a.rpc(func(ctx context.Context, c *client.Client) error {
		bundle, err := c.Config(ctx)
		if err != nil {
			return err
		}
		out.settings = settingsFromWire(bundle.Settings.Settings)
		out.warnings = append(out.warnings, bundle.Providers.Warnings...)
		out.warnings = append(out.warnings, bundle.MCP.Warnings...)
		for _, p := range bundle.Providers.Providers {
			out.providers = append(out.providers, providerFromWire(p))
		}
		out.info = map[string]mcpInfo{}
		for _, s := range bundle.MCP.Servers {
			out.servers = append(out.servers, serverFromWire(s))
			out.info[s.Name] = mcpInfo{Running: s.Running, Starting: s.Starting, Err: s.Error}
		}
		for _, k := range bundle.Skills.Skills {
			out.skills = append(out.skills, config.Skill{Name: k.Name, Scope: k.Scope, Desc: k.Desc, Path: k.Path, Enabled: k.Enabled})
		}
		return nil
	})
	return out
}

func settingsFromWire(w api.ConfigSettings) config.Settings {
	s := config.DefaultSettings()
	b, _ := json.Marshal(w)
	_ = json.Unmarshal(b, &s)
	return s
}

func settingsToWire(s config.Settings) api.ConfigSettings {
	var w api.ConfigSettings
	b, _ := json.Marshal(s)
	_ = json.Unmarshal(b, &w)
	return w
}

func providerFromWire(w api.ConfigProvider) llm.Provider {
	b, _ := json.Marshal(w)
	var p llm.Provider
	_ = json.Unmarshal(b, &p)
	p.Source = w.Source
	if w.HasAPIKey && p.APIKey == "" {
		p.APIKey = "•" // the daemon keeps the key; the UI only needs to know there is one
	}
	return p
}

func providerToWire(p llm.Provider) api.ConfigProvider {
	b, _ := json.Marshal(p)
	var w api.ConfigProvider
	_ = json.Unmarshal(b, &w)
	w.Source = p.Source
	if p.APIKey == "•" {
		w.APIKey = ""
	}
	return w
}

func serverFromWire(w api.ConfigMCPServer) mcp.Server {
	return mcp.Server{Name: w.Name, Transport: w.Transport, Command: w.Command, Args: w.Args, Cwd: w.Cwd, Env: w.Env, URL: w.URL, Headers: w.Headers, Enabled: w.Enabled}
}

func serverToWire(s mcp.Server) api.ConfigMCPServer {
	return api.ConfigMCPServer{Name: s.Name, Transport: s.Transport, Command: s.Command, Args: s.Args, Cwd: s.Cwd, Env: s.Env, URL: s.URL, Headers: s.Headers, Enabled: s.Enabled}
}

// reportWarnings shows what was wrong at start, once.
func (a *App) reportWarnings() {
	for _, w := range a.warnings {
		a.notifyErr(w)
	}
	a.warnings = nil
}

var _ = fmt.Sprint

// showFileWarnings tells about a provider.json or mcp.json that has a broken
// entry or is not valid JSON, once per distinct problem.
func (a *App) showFileWarnings(warns []string) {
	text := strings.Join(warns, "\n")
	if text == a.fileWarning {
		return
	}
	a.fileWarning = text
	if text != "" {
		a.notifyErr(text)
	}
}
