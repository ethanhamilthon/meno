package jsvm

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

type Shell struct {
	Gate    *Gate
	Timeout time.Duration
	MaxOut  int
}

type ShResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

func (s Shell) Run(ctx context.Context, cmd string) (ShResult, error) {
	if err := s.Gate.CheckCommand(cmd); err != nil {
		return ShResult{}, err
	}
	c := context.Background()
	var cancel context.CancelFunc
	if s.Timeout > 0 {
		c, cancel = context.WithTimeout(c, s.Timeout)
		defer cancel()
	}
	c, stop := context.WithCancel(c)
	defer stop()
	go func() {
		select {
		case <-ctx.Done():
			stop()
		case <-c.Done():
		}
	}()
	e := exec.CommandContext(c, "/bin/sh", "-c", cmd)
	e.Dir = s.Gate.Root
	var out, errb bytes.Buffer
	e.Stdout = &out
	e.Stderr = &errb
	err := e.Run()
	res := ShResult{Stdout: truncate(out.String(), s.MaxOut), Stderr: truncate(errb.String(), s.MaxOut)}
	if c.Err() == context.DeadlineExceeded {
		res.TimedOut = true
	}
	if ee, ok := err.(*exec.ExitError); ok {
		res.ExitCode = ee.ExitCode()
		return res, nil
	}
	return res, err
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n…[truncated]"
}
