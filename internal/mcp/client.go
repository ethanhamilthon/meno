package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"meno/internal/safe"
	"os/exec"
	"strings"
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
	// errs keeps the stderr of a stdio server for the error message.
	errs *stderrBuffer
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
	ctx, cancel := context.WithCancel(ctx)
	cmd, errs, err := stdioProcess(ctx, s)
	if err != nil {
		cancel()
		return nil, err
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
	c := &Client{Server: s, cmd: cmd, conn: newConn(stdin), done: make(chan struct{}), ctx: ctx, cancel: cancel, errs: errs}
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
		return nil, c.withStderr(err)
	}
	return c, nil
}

// withStderr adds what the server wrote to stderr to a startup error.
func (c *Client) withStderr(err error) error {
	if c.errs == nil {
		return err
	}
	text := strings.TrimSpace(c.errs.String())
	if text == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, text)
}

// wait waits for the answer to one request. what names the request in the
// timeout message, so a stuck server says which step did not answer.
func (c *Client) wait(ctx context.Context, ch chan rpcMsg, id int64, timeout time.Duration, what string) (json.RawMessage, error) {
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
		return nil, fmt.Errorf("mcp: no answer to %s within %s", what, timeout)
	case <-ctx.Done():
		return nil, fmt.Errorf("mcp: no answer to %s: %w", what, ctx.Err())
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
