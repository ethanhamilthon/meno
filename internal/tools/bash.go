package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// RunOptions contains the inputs for one fresh bash process.
//
// An empty Env starts with no inherited variables. Callers must include all
// needed host and MENO_* values.
type RunOptions struct {
	Command string
	Dir     string
	// Env replaces the inherited environment. It must include caller and MENO_* values.
	Env []string
	// OutputDir is the per-session directory below $MENO_HOME/outputs.
	OutputDir string
	CallID    string
	// Timeout is zero for no timeout; positive values start the call timer.
	Timeout time.Duration
	// Stream receives raw output deltas in pipe order.
	Stream func(delta string)
}

// Result describes the complete, user-facing result of a bash call.
type Result struct {
	// Text is output followed by status and any leftover-process suffix.
	Text string
	// Output is the output text, with a head/tail cut marker when needed.
	Output string
	// Status is the exact status line, including the optional cleanup suffix.
	Status    string
	ExitCode  int
	LogPath   string
	Omitted   int64
	Leftovers int
	TimedOut  bool
	Cancelled bool
}

type pauseChange struct {
	paused bool
	at     time.Time
}

// Call is a running bash call. Pause and Resume control only its timer.
type Call struct {
	cmd       *exec.Cmd
	options   RunOptions
	cancel    context.CancelFunc
	paused    bool
	mu        sync.Mutex
	changed   chan pauseChange
	finished  chan struct{}
	collected chan struct{}
	result    Result
	err       error
}

// Start starts /bin/sh -c in a new process group. A zero timeout disables the
// call timer. The returned handle lets an open ask pause the timer.
func Start(ctx context.Context, options RunOptions) (*Call, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if options.Dir == "" {
		return nil, fmt.Errorf("working directory is required")
	}
	if options.Command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if options.Timeout < 0 {
		return nil, fmt.Errorf("timeout cannot be negative")
	}
	cmd := exec.Command("/bin/sh", "-c", options.Command)
	return start(ctx, options, cmd)
}

func start(ctx context.Context, options RunOptions, cmd *exec.Cmd) (*Call, error) {
	if options.Dir == "" {
		return nil, fmt.Errorf("working directory is required")
	}
	if options.Timeout < 0 {
		return nil, fmt.Errorf("timeout cannot be negative")
	}
	log, path, err := createLog(options.OutputDir, options.CallID)
	if err != nil {
		return nil, err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		log.file.Close()
		return nil, err
	}
	cmd.Dir, cmd.Stdin = options.Dir, nil
	cmd.Stdout, cmd.Stderr = writer, writer
	cmd.Env = commandEnv(options.Env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		reader.Close()
		writer.Close()
		log.file.Close()
		return nil, err
	}
	writer.Close()
	callCtx, cancel := context.WithCancel(ctx)
	call := &Call{cmd: cmd, options: options, cancel: cancel, changed: make(chan pauseChange, 256), finished: make(chan struct{}), collected: make(chan struct{})}
	go call.collect(reader, log, path)
	go call.wait(callCtx)
	return call, nil
}

func commandEnv(extra []string) []string {
	values := make(map[string]string)
	for _, item := range extra {
		if key, value, ok := strings.Cut(item, "="); ok && key != "" {
			values[key] = value
		}
	}
	for key, value := range map[string]string{"TERM": "dumb", "NO_COLOR": "1", "PAGER": "cat", "GIT_PAGER": "cat"} {
		values[key] = value
	}
	out := make([]string, 0, len(values))
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}

// Pause stops the call timer until Resume. It returns false after completion.
func (c *Call) Pause() bool { return c.setPaused(true) }

// Resume restarts the call timer after Pause. It returns false after completion.
func (c *Call) Resume() bool { return c.setPaused(false) }

// Cancel stops the process group and its descendants.
func (c *Call) Cancel() { c.cancel() }

func (c *Call) setPaused(paused bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.finished:
		return false
	default:
	}
	if c.paused == paused {
		return true
	}
	change := pauseChange{paused: paused, at: time.Now()}
	select {
	case c.changed <- change:
		c.paused = paused
		return true
	case <-c.finished:
		return false
	}
}

// Wait blocks until the shell exits and its process group is stopped.
func (c *Call) Wait() (Result, error) { <-c.finished; return c.result, c.err }
