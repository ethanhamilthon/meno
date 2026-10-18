package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"meno/internal/safe"
	"os"
	"os/exec"
	"strings"
	"time"
)

// callTimeout is the longest a tool call may take; the caller's context ends
// it sooner.
const callTimeout = 10 * time.Minute

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type Client struct {
	Server Server
	cmd    *exec.Cmd
	closer func() // set by remote transports
	conn   *conn
	Tools  []Tool
	done   chan struct{}
}

// parseCommand supports shell-style quoting and escaping, without invoking a
// shell or expanding variables, substitutions, globs, or operators.
func parseCommand(command string) ([]string, error) {
	var args []string
	var word strings.Builder
	var quote byte
	started := false
	for i := 0; i < len(command); i++ {
		c := command[i]
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		if c == '\\' {
			if i+1 == len(command) {
				return nil, errors.New("mcp: trailing command escape")
			}
			next := command[i+1]
			if quote == '"' && next != '"' && next != '\\' && next != '$' && next != '`' && next != '\n' {
				word.WriteByte(c)
				continue
			}
			i++
			if next != '\n' {
				word.WriteByte(next)
				started = true
			}
			continue
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			started = true
		case ' ', '\t', '\n', '\r':
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
		default:
			word.WriteByte(c)
			started = true
		}
	}
	if quote != 0 {
		return nil, errors.New("mcp: unterminated command quote")
	}
	if started {
		args = append(args, word.String())
	}
	if len(args) == 0 || args[0] == "" {
		return nil, errors.New("mcp: empty command")
	}
	return args, nil
}

func Start(ctx context.Context, s Server) (*Client, error) {
	switch s.Transport {
	case TransportHTTP:
		return startHTTP(ctx, s)
	case TransportSSE:
		return startSSE(ctx, s)
	case TransportStdio:
	default:
		return nil, fmt.Errorf("unknown transport %q", s.Transport)
	}
	parts, err := parseCommand(s.Command)
	if err != nil {
		return nil, err
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
		defer safe.Recover(nil)
		_ = c.conn.dispatch(stdout)
		close(c.done)
	}()
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) wait(ctx context.Context, ch chan rpcMsg, id int64, timeout time.Duration) (json.RawMessage, error) {
	defer c.conn.pending.Delete(id)
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
	_, ch, id, err := c.conn.call("initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "meno", "version": "0.1"},
	})
	if err != nil {
		return err
	}
	if _, err := c.wait(ctx, ch, id, 10*time.Second); err != nil {
		return err
	}
	if err := c.conn.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return err
	}
	return c.refresh(ctx)
}

func (c *Client) refresh(ctx context.Context) error {
	_, ch, id, err := c.conn.call("tools/list", map[string]any{})
	if err != nil {
		return err
	}
	raw, err := c.wait(ctx, ch, id, 10*time.Second)
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
	_, ch, id, err := c.conn.call("tools/call", map[string]any{"name": tool, "arguments": args})
	if err != nil {
		return "", err
	}
	raw, err := c.wait(ctx, ch, id, callTimeout)
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
		Structured json.RawMessage `json:"structuredContent"`
		IsError    bool            `json:"isError"`
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
	if s == "" && len(r.Structured) > 0 && string(r.Structured) != "null" {
		s = string(r.Structured)
	}
	if r.IsError {
		return s, errors.New(s)
	}
	return s, nil
}

func (c *Client) Close() {
	if c.closer != nil {
		c.closer()
	}
	if c.cmd != nil {
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		_ = c.cmd.Wait()
	}
}

func (c *Client) Alive() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}
