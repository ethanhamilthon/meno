package app

import (
	"context"
	"time"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/proto"
)

// persistSettings sends the settings to the daemon, which stores them.
func (a *App) persistSettings() {
	a.cfgSeen = a.cfgSeq.Add(1) // reads begun before this edit are older than it
	s := a.st.Settings
	a.pushSave(func() {
		if err := a.rpc(func(ctx context.Context, c *client.Client) error {
			_, err := c.PutSettings(ctx, settingsToWire(s))
			return err
		}); err != nil {
			a.postEvent(errNotice{text: "settings: " + err.Error()})
		}
	})
}

// persistProviders sends every provider the user owns to the daemon.
func (a *App) persistProviders() {
	a.cfgSeen = a.cfgSeq.Add(1)
	own := config.OwnProviders(a.st.Providers)
	a.pushSave(func() {
		err := a.rpc(func(ctx context.Context, c *client.Client) error { return syncProviders(ctx, c, own) })
		if err != nil {
			a.postEvent(errNotice{text: "providers: " + err.Error()})
		}
		a.postEvent(a.fetchConfig())
	})
}

func syncProviders(ctx context.Context, c *client.Client, own []llm.Provider) error {
	cur, err := c.Providers(ctx)
	if err != nil {
		return err
	}
	have := map[string]proto.ConfigProvider{}
	for _, p := range cur.Providers {
		have[p.Name] = p
	}
	keep := map[string]bool{}
	for _, p := range own {
		keep[p.Name] = true
		w := providerToWire(p)
		if _, ok := have[p.Name]; ok {
			if _, err = c.UpdateProvider(ctx, p.Name, w); err != nil {
				return err
			}
		} else if _, err = c.CreateProvider(ctx, w); err != nil {
			return err
		}
	}
	for name, p := range have {
		if !keep[name] && p.Source != llm.SourceFile && p.Source != llm.SourceOAuth {
			if err = c.DeleteProvider(ctx, name); err != nil {
				return err
			}
		}
	}
	return nil
}

// persistServers sends the MCP servers the user owns to the daemon.
func (a *App) persistServers() {
	a.cfgSeen = a.cfgSeq.Add(1)
	own := config.OwnServers(a.st.Servers)
	a.pushSave(func() {
		err := a.rpc(func(ctx context.Context, c *client.Client) error { return syncServers(ctx, c, own) })
		if err != nil {
			a.postEvent(errNotice{text: "mcp: " + err.Error()})
		}
		a.postEvent(a.fetchConfig())
	})
}

func syncServers(ctx context.Context, c *client.Client, own []mcp.Server) error {
	cur, err := c.McpServers(ctx)
	if err != nil {
		return err
	}
	have := map[string]proto.ConfigMCPServer{}
	for _, s := range cur.Servers {
		have[s.Name] = s
	}
	keep := map[string]bool{}
	for _, s := range own {
		keep[s.Name] = true
		w := serverToWire(s)
		if _, ok := have[s.Name]; ok {
			if _, err = c.UpdateMcpServer(ctx, s.Name, w); err != nil {
				return err
			}
		} else if _, err = c.AddMcpServer(ctx, w); err != nil {
			return err
		}
	}
	for name, s := range have {
		if !keep[name] && s.Source != mcp.SourceFile {
			if err = c.RemoveMcpServer(ctx, name); err != nil {
				return err
			}
		}
	}
	return nil
}

// persistSkills sends the on/off state of the skills to the daemon.
func (a *App) persistSkills() {
	skills := append([]config.Skill(nil), a.st.Skills...)
	a.pushSave(func() {
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			for _, s := range skills {
				if _, err := c.SetSkillEnabled(ctx, s.Name, s.Enabled); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			a.postEvent(errNotice{text: "skills: " + err.Error()})
		}
	})
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + "h ago"
	}
	return itoa(int(d.Hours()/24)) + "d ago"
}
