// Package client provides typed calls and resumable streams for the meno daemon.
package client

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"meno/internal/api"
	"meno/internal/daemon"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Client communicates with one daemon over its unix socket.
type Client struct {
	Home, Socket string
	HTTP         *http.Client
	// IdleTimeout closes an event stream that sends nothing (not even a
	// keepalive) for this long, so the stream reconnects. Zero means 45s.
	IdleTimeout time.Duration
}

// Options configures daemon discovery and startup. StartTimeout defaults to five seconds.
type Options struct {
	Home, Version, Executable string
	StartTimeout              time.Duration
}

// New creates an HTTP client for the daemon socket.
func New(home string) *Client {
	sock := socketPath(home)
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", sock)
	}}
	return &Client{Home: home, Socket: sock, HTTP: &http.Client{Transport: tr}}
}

// Ensure connects to a compatible daemon or starts one under the startup lock.
func Ensure(ctx context.Context, o Options) (*Client, error) {
	if o.Home == "" {
		return nil, errors.New("MENO_HOME or home path is required")
	}
	transition, err := daemon.AcquireTransition(ctx, o.Home)
	if err != nil {
		return nil, fmt.Errorf("waiting for update/startup: %w", err)
	}
	defer transition.Close()
	if err := daemon.CheckExecutable(); err != nil {
		return nil, err
	}
	c := New(o.Home)
	if e := c.check(ctx); e == nil {
		return c, nil
	} else if strings.Contains(e.Error(), "protocol mismatch") {
		return nil, e
	}
	run := filepath.Join(o.Home, "run")
	if e := os.MkdirAll(run, 0o700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(run, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if e == nil {
			break
		}
		if ce := c.check(ctx); ce == nil {
			return c, nil
		} else if strings.Contains(ce.Error(), "protocol mismatch") {
			return nil, ce
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for daemon startup lock")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if e = c.check(ctx); e == nil {
		return c, nil
	} else if strings.Contains(e.Error(), "protocol mismatch") {
		return nil, e
	}
	exe := o.Executable
	if exe == "" {
		exe, e = os.Executable()
		if e != nil {
			return nil, e
		}
	}
	cmd := exec.Command(exe, "daemon", "run")
	cmd.Env = append(daemonEnv(os.Environ(), o.Home), "MENO_DAEMON_LOCK_FD=3")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.ExtraFiles = []*os.File{f}
	logDir := filepath.Join(o.Home, "log")
	if e = os.MkdirAll(logDir, 0o700); e != nil {
		return nil, e
	}
	logPath := filepath.Join(logDir, "daemon.log")
	if e = rotateClientLog(logPath); e != nil {
		return nil, e
	}
	lf, e := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if e != nil {
		return nil, e
	}
	cmd.Stdout = lf
	cmd.Stderr = lf
	if e = cmd.Start(); e != nil {
		lf.Close()
		return nil, e
	}
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
	_ = cmd.Process.Release()
	_ = lf.Close()
	timeout := o.StartTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline = time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if e = c.check(ctx); e == nil {
			return c, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(40 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("daemon did not start within %s", timeout)
}
func (c *Client) check(ctx context.Context) error {
	var h api.Hello
	if e := c.get(ctx, "/v1/hello", &h); e != nil {
		return e
	}
	if h.Protocol != api.Protocol {
		return fmt.Errorf("protocol mismatch: client %d, daemon %d; run meno daemon restart", api.Protocol, h.Protocol)
	}
	return nil
}
