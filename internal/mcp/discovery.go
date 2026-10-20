package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"meno/internal/version"
	"time"
)

func (c *Client) request(ctx context.Context, method string, params any, timeout time.Duration) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	id := c.conn.nextID.Add(1)
	ch := make(chan rpcMsg, 1)
	c.conn.pending.Store(id, ch)
	defer c.conn.pending.Delete(id)
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	sent := make(chan error, 1)
	go func() {
		if c.sendRemote != nil {
			sent <- c.sendRemote(ctx, msg)
		} else {
			sent <- c.conn.send(msg)
		}
	}()
	select {
	case err := <-sent:
		if err != nil {
			return nil, err
		}
	case reply := <-ch:
		if reply.Error != nil {
			return nil, reply.Error
		}
		return reply.Result, nil
	case <-ctx.Done():
		if c.sendRemote == nil {
			c.Close()
		}
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("mcp: server exited")
	}
	return c.wait(ctx, ch, id, timeout)
}

func (c *Client) initialize(ctx context.Context) error {
	raw, err := c.request(ctx, "initialize", map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "meno", "version": version.Number()}}, 10*time.Second)
	if err != nil {
		return err
	}
	var init struct {
		Version string `json:"protocolVersion"`
	}
	if err = json.Unmarshal(raw, &init); err != nil {
		return err
	}
	switch init.Version {
	case "2024-11-05", "2025-03-26", protocolVersion:
	default:
		return fmt.Errorf("mcp: unsupported protocol version %q", init.Version)
	}
	if c.setVersion != nil {
		c.setVersion(init.Version)
	}
	msg := map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}
	if c.sendRemote != nil {
		err = c.sendRemote(ctx, msg)
	} else {
		err = c.conn.send(msg)
	}
	if err != nil {
		return err
	}
	return c.refresh(ctx)
}

func (c *Client) refresh(ctx context.Context) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	tools := []Tool{}
	cursor := ""
	seen := map[string]bool{}
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.request(ctx, "tools/list", params, 10*time.Second)
		if err != nil {
			return err
		}
		var out struct {
			Tools []Tool `json:"tools"`
			Next  string `json:"nextCursor"`
		}
		if err = json.Unmarshal(raw, &out); err != nil {
			return err
		}
		tools = append(tools, out.Tools...)
		if out.Next == "" {
			break
		}
		if seen[out.Next] {
			return fmt.Errorf("mcp: repeated tools cursor")
		}
		seen[out.Next] = true
		cursor = out.Next
	}
	c.toolsMu.Lock()
	c.tools = tools
	c.toolsMu.Unlock()
	return nil
}

func (c *Client) ToolSnapshot() []Tool {
	c.toolsMu.RLock()
	defer c.toolsMu.RUnlock()
	return append([]Tool(nil), c.tools...)
}

func (c *Client) notification(method string) {
	if method == "notifications/tools/list_changed" {
		go func() { _ = c.refresh(c.ctx) }()
	}
}
