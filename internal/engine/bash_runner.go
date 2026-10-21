package engine

import (
	"os"
	"path/filepath"
	"strconv"

	"meno/internal/agent"
	"meno/internal/proc"
	"meno/internal/tools"
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
	o := proc.LaunchOptions{
		Session: b.s.session.ID, Owner: owner, Name: "bash " + id, Command: command, Cwd: b.cwd(), Env: b.env(id),
		CallID: id, OutputDir: filepath.Join(b.e.home, "outputs", b.s.session.ID),
	}
	if allowed := []string(nil); allowed != nil {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		parsed, err := tools.ParseRestrictedCommand(command, exe, allowed)
		if err != nil {
			return nil, err
		}
		o.Path, o.Args = parsed.Path, parsed.Args
	}
	fg, err := b.e.tasks.Launch(o)
	if err != nil {
		return nil, err
	}
	return &fgCall{e: b.e, session: b.s.session.ID, fg: fg}, nil
}

// fgCall is a running agent.RunningCall on the supervisor.
type fgCall struct {
	e       *Engine
	session string
	fg      *proc.Foreground
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
