package tui

import (
	"context"
	"time"

	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/llm"
)

// persistSettings sends the fields that changed to the daemon. A field that
// someone else changed meanwhile is a conflict: the user sees it and the
// shared value is loaded again instead of being overwritten.
func (a *App) persistSettings() {
	a.cfgSeen = a.cfgSeq.Add(1) // reads begun before this edit are older than it
	fields, expect := settingsDelta(a.cfgBase, a.sharedSettings())
	if len(fields) == 0 {
		return
	}
	a.cfgBase = a.sharedSettings() // the next edit is measured from this one
	a.pushSave(func() {
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			_, err := c.PatchSettings(ctx, fields, expect)
			return err
		})
		if err != nil {
			a.postEvent(errNotice{text: "settings: " + err.Error()})
			a.postEvent(a.fetchConfig())
		}
	})
}

// persistProviders sends every provider the user owns to the daemon.
func (a *App) persistProviders() {
	a.cfgSeen = a.cfgSeq.Add(1)
	// Only entries this client changed are sent; the others may have been
	// edited elsewhere and must not be overwritten by a stale copy.
	var own []llm.Provider
	for _, p := range config.OwnProviders(a.st.Providers) {
		if old, ok := a.cfgProv[p.Name]; !ok || string(mustJSON(providerToWire(old))) != string(mustJSON(providerToWire(p))) {
			own = append(own, p)
		}
	}
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
	have := map[string]api.ConfigProvider{}
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
	// A provider that is missing from the list is never deleted here: an
	// absent entry may simply be one this client has not seen yet. Deletion
	// is an explicit request (deleteProvider).
	_ = keep
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
