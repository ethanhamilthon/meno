package app

import (
	"context"

	"meno/internal/client"
	"meno/internal/mcp"
	"meno/internal/proto"
	"time"
)

// restartMCP asks the daemon to restart one MCP server, then re-reads the list.
func (a *App) restartMCP(name string) {
	a.pushSave(func() {
		if err := a.rpc(func(ctx context.Context, c *client.Client) error { return c.RestartMcpServer(ctx, name) }); err != nil {
			a.postEvent(errNotice{text: "mcp " + name + ": " + err.Error()})
		}
		a.postEvent(a.fetchConfig())
	})
}

func (a *App) saveMCPServer(server mcp.Server, isNew bool) {
	a.cfgSeen = a.cfgSeq.Add(1)
	a.pushSave(func() {
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			if isNew {
				_, err := c.AddMcpServer(ctx, serverToWire(server))
				return err
			}
			_, err := c.UpdateMcpServer(ctx, server.Name, serverToWire(server))
			return err
		})
		if err != nil {
			a.postEvent(errNotice{text: "mcp: " + err.Error()})
		}
		a.postEvent(a.fetchConfig())
	})
}
func (a *App) deleteMCPServer(name string) {
	a.cfgSeen = a.cfgSeq.Add(1)
	a.pushSave(func() {
		if err := a.rpc(func(ctx context.Context, c *client.Client) error { return c.RemoveMcpServer(ctx, name) }); err != nil {
			a.postEvent(errNotice{text: "mcp: " + err.Error()})
		}
		a.postEvent(a.fetchConfig())
	})
}

type mcpStatusLoaded struct {
	servers  []proto.ConfigMCPServer
	err      error
	warnings []string
}

func (a *App) refreshMCPStatus() {
	if a.st.Over != OverMCP && a.st.Over != OverMCPEdit {
		return
	}
	if a.mcpPending || time.Since(a.mcpRefresh) < time.Second {
		return
	}
	a.mcpPending = true
	a.mcpRefresh = time.Now()
	a.spawn(func() {
		var servers []proto.ConfigMCPServer
		var warnings []string
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			r, err := c.McpServers(ctx)
			servers = r.Servers
			warnings = r.Warnings
			return err
		})
		a.postEvent(mcpStatusLoaded{servers: servers, err: err, warnings: warnings})
	})
}
