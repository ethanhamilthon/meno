package tools

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"
)

func (c *Call) wait(ctx context.Context) {
	defer c.markFinished()
	defer c.cancel()
	exited := make(chan error, 1)
	go func() { exited <- c.cmd.Wait() }()
	var timer *time.Timer
	var timeout <-chan time.Time
	remaining := c.options.Timeout
	timerPaused := false
	if remaining > 0 {
		timer = time.NewTimer(remaining)
		timeout = timer.C
	}
	started := time.Now()
	var waitErr error
	for {
		select {
		case waitErr = <-exited:
			goto done
		case <-ctx.Done():
			c.result.Cancelled = true
			terminateGroup(c.cmd.Process.Pid, false)
			waitErr = <-exited
			goto done
		case <-timeout:
			c.result.TimedOut = true
			terminateGroup(c.cmd.Process.Pid, true)
			waitErr = <-exited
			goto done
		case change := <-c.changed:
			if change.paused {
				if timer != nil && !timerPaused {
					remaining -= change.at.Sub(started)
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timeout = nil
					timerPaused = true
				}
			} else {
				if timer != nil && timerPaused {
					started = change.at
					timer.Reset(max(remaining, time.Nanosecond))
					timeout = timer.C
					timerPaused = false
				}
			}
		}
	}
done:
	if timer != nil && !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	c.result.Leftovers = killLeftovers(c.cmd.Process.Pid)
	if ctx.Err() != nil && !c.result.TimedOut {
		c.result.Cancelled = true
	}
	c.result.ExitCode = processExitCode(waitErr)
	switch {
	case c.result.TimedOut:
		seconds := int(c.options.Timeout / time.Second)
		if c.options.Timeout%time.Second != 0 {
			seconds++
		}
		c.result.Status = fmt.Sprintf("[timed out after %ds]", seconds)
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

func (c *Call) markFinished() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.finished)
}

func terminateGroup(pgid int, grace bool) {
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	if !grace {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(processGroupPIDs(pgid)) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
