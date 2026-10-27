package engine

import (
	"net"
	"os"
	"path/filepath"
	"strconv"

	"meno/internal/agent"
	"meno/internal/callfd"
	"meno/internal/process"
)

// bashRunner is the agent.ToolRunner of the bash tool: one fresh /bin/sh per
// call, or, when the mode restricts the groups, the daemon's own meno binary
// with a parsed argv and no shell. The process supervisor owns every process
// from its launch, so promoting a call to a task never starts anything again.
type bashRunner struct {
	e *Engine
	s *runState
}

func (e *Engine) newBashRunner(s *runState) *bashRunner { return &bashRunner{e: e, s: s} }

// Start launches one call and returns at once.
func (b *bashRunner) Start(id, command string) (agent.RunningCall, error) {
	if id == "" {
		id = agent.NewID()
	}
	b.s.mu.Lock()
	owner := b.s.requestID
	b.s.mu.Unlock()
	var ch *callfd.Channel
	if b.e.serveCall != nil {
		b.s.mu.Lock()
		token := b.s.token.value
		b.s.mu.Unlock()
		var err error
		if ch, err = callfd.Open(func(c net.Conn) { b.e.serveCall(c, token, id) }); err != nil {
			return nil, err
		}
	}
	o := process.LaunchOptions{
		Session: b.s.session.ID, Owner: owner, Name: "bash " + id, Command: command, Cwd: b.cwd(), Env: b.env(id, ch != nil),
		CallID: id, OutputDir: filepath.Join(b.e.home, "outputs", b.s.session.ID),
	}
	if ch != nil {
		o.ExtraFiles = ch.Files()
	}
	if allowed := []string(nil); allowed != nil {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		parsed, err := process.ParseRestrictedCommand(command, exe, allowed)
		if err != nil {
			return nil, err
		}
		o.Path, o.Args = parsed.Path, parsed.Args
	}
	fg, err := b.e.tasks.Launch(o)
	if ch != nil {
		ch.ReleaseChild()
	}
	if err != nil {
		if ch != nil {
			ch.Close()
		}
		return nil, err
	}
	if ch != nil {
		go func() { <-fg.Done(); ch.Close() }()
	}
	return &fgCall{e: b.e, session: b.s.session.ID, fg: fg}, nil
}

// fgCall is a running agent.RunningCall on the supervisor.
type fgCall struct {
	e       *Engine
	session string
	fg      *process.Foreground
}

func (c *fgCall) Done() <-chan struct{} { return c.fg.Done() }

func (c *fgCall) Result() agent.ToolResult {
	res, err := c.fg.Result()
	if err != nil {
		return agent.ToolResult{Output: res.Text, Err: err.Error()}
	}
	return agent.ToolResult{Output: res.Text}
}

func (c *fgCall) Cancel() { c.fg.Cancel() }

func (c *fgCall) Promote() (string, bool) {
	t, ok := c.fg.Promote()
	if ok {
		c.e.publishTasksChanged(c.session)
	}
	return t.ID, ok
}

func (b *bashRunner) cwd() string {
	b.s.mu.Lock()
	defer b.s.mu.Unlock()
	return b.s.cwd
}

func (b *bashRunner) env(call string, channel bool) []string {
	b.s.mu.Lock()
	env := envSlice(b.s.env)
	token, id := b.s.token.value, b.s.session.ID
	b.s.mu.Unlock()
	if len(b.s.env) == 0 {
		env = append(env, os.Environ()...)
	}
	env = append(env,
		"MENO_HOME="+b.e.home, "MENO_SESSION="+id,
		"MENO_TOKEN="+token, "MENO_CALL="+call, "MENO_PID="+strconv.Itoa(os.Getpid()))
	if channel {
		env = append(env, "MENO_FD="+strconv.Itoa(callfd.FD))
	}
	return withMenoPath(env, b.e.binDir())
}
