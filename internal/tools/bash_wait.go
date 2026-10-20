package tools

import (
	"context"
	"fmt"
	"strings"
	"syscall"
)

func (c *Call) wait(ctx context.Context) {
	defer c.markFinished()
	defer c.cancel()
	exited := make(chan error, 1)
	go func() { exited <- c.cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-exited:
	case <-ctx.Done():
		c.result.Cancelled = true
		terminateGroup(c.cmd.Process.Pid, false)
		waitErr = <-exited
	}
	c.result.Leftovers = killLeftovers(c.cmd.Process.Pid)
	c.result.ExitCode = processExitCode(waitErr)
	switch {
	case c.result.Cancelled:
		c.result.Status = "[cancelled]"
	default:
		c.result.Status = fmt.Sprintf("[exit %d]", c.result.ExitCode)
	}
	if c.result.Leftovers > 0 {
		c.result.Status += fmt.Sprintf("\n[killed %d leftover processes; use meno tasks start for background work]", c.result.Leftovers)
	}
	<-c.collected
	if c.err == nil && waitErr != nil && c.result.ExitCode == 0 {
		c.err = waitErr
	}
	c.result.Text = c.result.Output
	if c.result.Text != "" && !strings.HasSuffix(c.result.Text, "\n") {
		c.result.Text += "\n"
	}
	c.result.Text += c.result.Status
}

func (c *Call) markFinished() { close(c.finished) }

func terminateGroup(pgid int, _ bool) {
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
