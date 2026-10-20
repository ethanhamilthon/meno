package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
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
	Cancelled bool
}

// Call is a running bash call. Its context is its own: nothing but Cancel stops
// it, so a call that is promoted to a task outlives the request that began it.
type Call struct {
	cmd       *exec.Cmd
	options   RunOptions
	cancel    context.CancelFunc
	finished  chan struct{}
	collected chan struct{}
	result    Result
	err       error
}

// Start starts /bin/sh -c in a new process group. There is no timeout.
func Start(options RunOptions) (*Call, error) {
	if options.Dir == "" {
		return nil, fmt.Errorf("working directory is required")
	}
	if options.Command == "" {
		return nil, fmt.Errorf("command is required")
	}
	return start(options, exec.Command("/bin/sh", "-c", options.Command))
}

func start(options RunOptions, cmd *exec.Cmd) (*Call, error) {
	if options.Dir == "" {
		return nil, fmt.Errorf("working directory is required")
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
	callCtx, cancel := context.WithCancel(context.Background())
	call := &Call{cmd: cmd, options: options, cancel: cancel, finished: make(chan struct{}), collected: make(chan struct{})}
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

// Cancel stops the process group and its descendants.
func (c *Call) Cancel() { c.cancel() }

// Done is closed when the shell has exited and its process group is stopped.
func (c *Call) Done() <-chan struct{} { return c.finished }

// Pid is the process ID of the shell, which is also its process group ID.
func (c *Call) Pid() int { return c.cmd.Process.Pid }

// ExitCode is the exit code of the shell; it is valid once Done is closed.
func (c *Call) ExitCode() int { <-c.finished; return c.result.ExitCode }

// Wait blocks until the shell exits and its process group is stopped.
func (c *Call) Wait() (Result, error) { <-c.finished; return c.result, c.err }
