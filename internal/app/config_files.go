package app

import (
	"context"
	"encoding/json"
	"fmt"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/proto"
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

// settingsLoaded carries the daemon's configuration to the UI goroutine.
type settingsLoaded struct {
	seq       uint64 // order of the read: later reads win
	settings  config.Settings
	providers []llm.Provider
	servers   []mcp.Server
	info      map[string]mcpInfo
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
	a.spawn(func() { a.postEvent(a.fetchConfig()) })
}

func (a *App) onSettingsLoaded(m settingsLoaded) {
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
	a.fetchModels()
	a.afterProvidersChanged()
}

func (a *App) applyConfig(m settingsLoaded) {
	a.st.Settings, a.st.Providers, a.st.Servers, a.st.Skills = m.settings, m.providers, m.servers, m.skills
	a.st.ServerInfo = m.info
}

func (a *App) fetchConfig() settingsLoaded {
	out := settingsLoaded{seq: a.cfgSeq.Add(1)}
	out.err = a.rpc(func(ctx context.Context, c *client.Client) error {
		s, err := c.Settings(ctx)
		if err != nil {
			return err
		}
		out.settings = settingsFromWire(s.Settings)
		ps, err := c.Providers(ctx)
		if err != nil {
			return err
		}
		for _, p := range ps.Providers {
			out.providers = append(out.providers, providerFromWire(p))
		}
		ms, err := c.McpServers(ctx)
		if err != nil {
			return err
		}
		out.info = map[string]mcpInfo{}
		for _, s := range ms.Servers {
			out.servers = append(out.servers, serverFromWire(s))
			out.info[s.Name] = mcpInfo{Running: s.Running, Starting: s.Starting, Err: s.Error}
		}
		sk, err := c.Skills(ctx)
		if err != nil {
			return err
		}
		for _, k := range sk.Skills {
			out.skills = append(out.skills, config.Skill{Name: k.Name, Scope: k.Scope, Desc: k.Desc, Path: k.Path, Enabled: k.Enabled})
		}
		return nil
	})
	return out
}

func settingsFromWire(w proto.ConfigSettings) config.Settings {
	s := config.DefaultSettings()
	b, _ := json.Marshal(w)
	_ = json.Unmarshal(b, &s)
	return s
}

func settingsToWire(s config.Settings) proto.ConfigSettings {
	var w proto.ConfigSettings
	b, _ := json.Marshal(s)
	_ = json.Unmarshal(b, &w)
	return w
}

func providerFromWire(w proto.ConfigProvider) llm.Provider {
	b, _ := json.Marshal(w)
	var p llm.Provider
	_ = json.Unmarshal(b, &p)
	p.Source = w.Source
	if w.HasAPIKey && p.APIKey == "" {
		p.APIKey = "•" // the daemon keeps the key; the UI only needs to know there is one
	}
	return p
}

func providerToWire(p llm.Provider) proto.ConfigProvider {
	b, _ := json.Marshal(p)
	var w proto.ConfigProvider
	_ = json.Unmarshal(b, &w)
	w.Source = p.Source
	if p.APIKey == "•" {
		w.APIKey = ""
	}
	return w
}

func serverFromWire(w proto.ConfigMCPServer) mcp.Server {
	return mcp.Server{Name: w.Name, Transport: w.Transport, Command: w.Command, Env: w.Env, URL: w.URL, Headers: w.Headers, Enabled: w.Enabled, Source: w.Source}
}

func serverToWire(s mcp.Server) proto.ConfigMCPServer {
	return proto.ConfigMCPServer{Name: s.Name, Transport: s.Transport, Command: s.Command, Env: s.Env, URL: s.URL, Headers: s.Headers, Enabled: s.Enabled, Source: s.Source}
}

// reportWarnings shows what was wrong at start, once.
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

var _ = fmt.Sprint
