package agent

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

// shellTool is a small ToolRunner for tests: each call runs /bin/sh -c and the
// output is what the shell printed. It stands in for the engine's bash runner.
type shellTool struct {
	dir      string
	mu       sync.Mutex
	calls    int
	started  map[string]*shellCall
	promoted []string
}

func newShellTool(dir string) *shellTool {
	return &shellTool{dir: dir, started: map[string]*shellCall{}}
}

type shellCall struct {
	t        *shellTool
	cmd      *exec.Cmd
	done     chan struct{}
	out      []byte
	err      error
	mu       sync.Mutex
	promoted bool
	finished bool
}

func (s *shellTool) Start(id, code string) (RunningCall, error) {
	cmd := exec.Command("/bin/sh", "-c", code)
	cmd.Dir = s.dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c := &shellCall{t: s, cmd: cmd, done: make(chan struct{})}
	s.mu.Lock()
	s.calls++
	s.started[id] = c
	s.mu.Unlock()
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		c.err = cmd.Wait()
		c.out = buf.Bytes()
		c.mu.Lock()
		c.finished = true
		c.mu.Unlock()
		close(c.done)
	}()
	return c, nil
}

func (c *shellCall) Done() <-chan struct{} { return c.done }

func (c *shellCall) Result() ToolResult {
	return ToolResult{Output: strings.TrimRight(string(c.out), "\n")}
}

func (c *shellCall) Cancel() {
	c.mu.Lock()
	keep := c.promoted
	c.mu.Unlock()
	if !keep && c.cmd.Process != nil {
		_ = syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
	}
}

func (c *shellCall) Promote() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.finished {
		return "", false
	}
	c.promoted = true
	c.t.mu.Lock()
	id := "t" + string(rune('a'+len(c.t.promoted)))
	c.t.promoted = append(c.t.promoted, id)
	c.t.mu.Unlock()
	return id, true
}
