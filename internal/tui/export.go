package tui

import (
	"context"
	"encoding/json"
	"path/filepath"

	"meno/internal/api"
	"meno/internal/client"
)

type exportDone struct {
	shown string
	err   error
}

func exportDestination(cwd string, args []string) (path, shown string) {
	name := "meno-session.md"
	if len(args) > 0 {
		name = args[0]
	}
	path, shown = name, name
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
		shown = "./" + name
	}
	return path, shown
}

func (a *App) exportSession(args []string) {
	path, shown := exportDestination(a.st.Cwd, args)
	sid := a.st.Session.ID()
	a.spawn(func() {
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			r, err := c.Resource(ctx, "GET", "/v1/sessions/"+sid+"/export", "", api.ResourceRequest{})
			if err != nil {
				return err
			}
			var export api.ExportResponse
			if err := json.Unmarshal(r.Data, &export); err != nil {
				return err
			}
			return writeFile(path, export.Markdown)
		})
		a.postEvent(exportDone{shown: shown, err: err})
	})
}

func (a *App) onExportDone(m exportDone) {
	if m.err != nil {
		a.notifyErr("export failed: " + m.err.Error())
		return
	}
	a.notify("session saved to " + m.shown)
}
