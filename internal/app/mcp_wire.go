package app

import (
	"context"

	"meno/internal/client"
)

// restartMCP asks the daemon to restart one MCP server, then re-reads the list.
func (a *App) restartMCP(name string) {
	a.spawn(func() {
		if err := a.rpc(func(ctx context.Context, c *client.Client) error { return c.RestartMcpServer(ctx, name) }); err != nil {
			a.postEvent(errNotice{text: "mcp " + name + ": " + err.Error()})
		}
		a.postEvent(a.fetchConfig())
	})
}
