package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"meno/internal/safe"
	"os"
	"os/exec"
	"sync"
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
	Server     Server
	cmd        *exec.Cmd
	closer     func() // set by remote transports
	conn       *conn
	tools      []Tool
	toolsMu    sync.RWMutex
	refreshMu  sync.Mutex
	closeOnce  sync.Once
	doneOnce   sync.Once
	ctx        context.Context
	cancel     context.CancelFunc
	sendRemote func(context.Context, any) error
	setVersion func(string)
	done       chan struct{}
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
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Env = os.Environ()
	for k, v := range s.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	c := &Client{Server: s, cmd: cmd, conn: newConn(stdin), done: make(chan struct{}), ctx: ctx, cancel: cancel}
	c.conn.notify = c.notification
	go func() {
		defer safe.Recover(nil)
		_ = c.conn.dispatch(stdout)
		cancel()
		_ = cmd.Wait()
		c.markDone()
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

func (c *Client) markDone() { c.doneOnce.Do(func() { close(c.done) }) }

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if c.closer != nil {
			c.closer()
		}
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		c.markDone()
	})
}

func (c *Client) Alive() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}
