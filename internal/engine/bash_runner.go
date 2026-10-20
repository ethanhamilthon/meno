package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"meno/internal/agent"
	"meno/internal/tools"
)

// bashRunner is the agent.ToolRunner of the bash tool: one fresh /bin/sh per
// call, or, when the mode restricts the groups, the daemon's own meno binary
// with a parsed argv and no shell.
type bashRunner struct {
	e       *Engine
	s       *runState
	timeout time.Duration
}

// newBashRunner returns the runner of a session. timeout is the default call limit.
func (e *Engine) newBashRunner(s *runState, timeout time.Duration) *bashRunner {
	return &bashRunner{e: e, s: s, timeout: timeout}
}

// Timeout is the default call limit (0 = none).
func (b *bashRunner) Timeout() time.Duration { return b.timeout }

// Run starts one call and returns its output; the status line ends the output.
func (b *bashRunner) Run(ctx context.Context, command string, limit time.Duration) (string, string, string, bool, time.Duration) {
	id := agent.CallID(ctx)
	if id == "" {
		id = agent.NewID()
	}
	start := time.Now()
	opts := tools.RunOptions{
		Command: command, Dir: b.cwd(), Env: b.env(id), CallID: id, Timeout: limit,
		OutputDir: filepath.Join(b.e.home, "outputs", b.s.session.ID),
		Stream:    func(string) {},
	}
	b.s.mu.Lock()
	b.s.calls[id] = nil
	b.s.mu.Unlock()
	var call *tools.Call
	var err error
	if allowed := b.s.modes.AllowedGroups(); allowed != nil {
		var exe string
		exe, err = os.Executable()
		if err == nil {
			call, err = tools.RunRestricted(ctx, command, exe, allowed, opts)
		}
	} else {
		call, err = tools.Start(ctx, opts)
	}
	if err != nil {
		b.forget(id)
		return "", "", err.Error(), false, time.Since(start)
	}
	b.hold(id, call)
	res, err := call.Wait()
	b.forget(id)
	elapsed := time.Since(start)
	if err != nil {
		return res.Text, "", err.Error(), false, elapsed
	}
	return res.Text, "", "", res.TimedOut, elapsed
}

func (b *bashRunner) cwd() string {
	b.s.mu.Lock()
	defer b.s.mu.Unlock()
	return b.s.cwd
}

func (b *bashRunner) env(call string) []string {
	b.s.mu.Lock()
	env := envSlice(b.s.env)
	token, id := b.s.token.value, b.s.session.ID
	b.s.mu.Unlock()
	if len(b.s.env) == 0 {
		env = append(env, os.Environ()...)
	}
	env = append(env,
		"MENO_HOME="+b.e.home, "MENO_SOCK="+b.e.sock, "MENO_SESSION="+id,
		"MENO_TOKEN="+token, "MENO_CALL="+call, "MENO_PID="+strconv.Itoa(os.Getpid()))
	return withMenoPath(env, b.e.binDir())
}

func (b *bashRunner) hold(id string, c *tools.Call) {
	b.s.mu.Lock()
	b.s.calls[id] = c
	b.s.mu.Unlock()
}

func (b *bashRunner) forget(id string) {
	b.s.mu.Lock()
	delete(b.s.calls, id)
	b.s.mu.Unlock()
}

// pauseCall stops or restarts the timer of a running call (an open ask).
func (s *runState) pauseCall(id string, pause bool) {
	s.mu.Lock()
	c := s.calls[id]
	s.mu.Unlock()
	if c == nil {
		return
	}
	if pause {
		c.Pause()
	} else {
		c.Resume()
	}
}
