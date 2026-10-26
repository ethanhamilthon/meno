package tui

import (
	"context"

	"meno/internal/client"
	"meno/internal/config"
)

// reloadSkills asks the daemon for the skills again; the project skills depend
// on the directory of the session.
func (a *App) reloadSkills() {
	cwd := a.st.Cwd
	a.spawn(func() {
		var out []config.Skill
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			r, e := c.Skills(ctx, cwd)
			for _, k := range r.Skills {
				out = append(out, config.Skill{Name: k.Name, Scope: k.Scope, Desc: k.Desc, Path: k.Path, Enabled: k.Enabled, Auto: k.Auto, Warnings: k.Warnings})
			}
			return e
		})
		if err == nil {
			a.postEvent(skillsLoaded{out, cwd})
		}
	})
}

// skillsLoaded is the answer for the directory cwd; one for a directory the
// window has left is dropped.
type skillsLoaded struct {
	skills []config.Skill
	cwd    string
}
