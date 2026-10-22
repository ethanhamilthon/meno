package tui

import (
	"context"

	"meno/internal/client"
	"meno/internal/config"
)

// reloadSkills asks the daemon for the skills again; the project skills depend
// on the directory of the session.
func (a *App) reloadSkills() {
	a.spawn(func() {
		var out []config.Skill
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			r, e := c.Skills(ctx)
			for _, k := range r.Skills {
				out = append(out, config.Skill{Name: k.Name, Scope: k.Scope, Desc: k.Desc, Path: k.Path, Enabled: k.Enabled})
			}
			return e
		})
		if err == nil {
			a.postEvent(skillsLoaded{out})
		}
	})
}

type skillsLoaded struct{ skills []config.Skill }
