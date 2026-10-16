package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type Client struct {
	Server Server
	cmd    *exec.Cmd
	conn   *conn
	Tools  []Tool
	done   chan struct{}
}

func Start(ctx context.Context, s Server) (*Client, error) {
	if s.Transport != TransportStdio {
		return nil, fmt.Errorf("transport %s not supported in this build", s.Transport)
	}
	parts := strings.Fields(s.Command)
	if len(parts) == 0 {
		return nil, errors.New("empty command")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Env = os.Environ()
	for k, v := range s.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &Client{Server: s, cmd: cmd, conn: newConn(stdin), done: make(chan struct{})}
	go func() {
		_ = c.conn.dispatch(stdout)
		close(c.done)
	}()
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) wait(ctx context.Context, ch chan rpcMsg, timeout time.Duration) (json.RawMessage, error) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case m := <-ch:
		if m.Error != nil {
			return nil, m.Error
		}
		return m.Result, nil
	case <-t.C:
		return nil, errors.New("mcp: timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, errors.New("mcp: server exited")
	}
}

func (c *Client) initialize(ctx context.Context) error {
	_, ch, _, err := c.conn.call("initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "meno", "version": "0.1"},
	})
	if err != nil {
		return err
	}
	if _, err := c.wait(ctx, ch, 10*time.Second); err != nil {
		return err
	}
	_ = c.conn.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	return c.refresh(ctx)
}

func (c *Client) refresh(ctx context.Context) error {
	_, ch, _, err := c.conn.call("tools/list", map[string]any{})
	if err != nil {
		return err
	}
	raw, err := c.wait(ctx, ch, 10*time.Second)
	if err != nil {
		return err
	}
	var out struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	c.Tools = out.Tools
	return nil
}

func (c *Client) Call(ctx context.Context, tool string, args map[string]any) (string, error) {
	_, ch, _, err := c.conn.call("tools/call", map[string]any{"name": tool, "arguments": args})
	if err != nil {
		return "", err
	}
	raw, err := c.wait(ctx, ch, 60*time.Second)
	if err != nil {
		return "", err
	}
	return ExtractText(raw)
}

func ExtractText(raw json.RawMessage) (string, error) {
	var r struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	var parts []string
	for _, c := range r.Content {
		if c.Type == "text" {
			parts = append(parts, c.Text)
		}
	}
	s := strings.Join(parts, "\n")
	if r.IsError {
		return s, errors.New(s)
	}
	return s, nil
}

func (c *Client) Close() {
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
}

func (c *Client) Alive() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}
