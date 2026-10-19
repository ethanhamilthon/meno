package jsvm

import (
	"bytes"
	"context"
	"meno/internal/safe"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

type Shell struct {
	Root    string
	Timeout time.Duration
	MaxOut  int
	// Env replaces the process environment when non-nil.
	Env []string
}

type ShResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

func (s Shell) Run(ctx context.Context, cmd string) (ShResult, error) {
	c := context.Background()
	var cancel context.CancelFunc
	if s.Timeout > 0 {
		c, cancel = context.WithTimeout(c, s.Timeout)
		defer cancel()
	}
	c, stop := context.WithCancel(c)
	defer stop()
	go func() {
		defer safe.Recover(nil)
		select {
		case <-ctx.Done():
			stop()
		case <-c.Done():
		}
	}()
	e := exec.CommandContext(c, "/bin/sh", "-c", cmd)
	e.Dir = s.Root
	if s.Env != nil {
		e.Env = s.Env
	}
	// Setsid gives the command its own session without a controlling terminal.
	// Setpgid alone left it in a background group of meno's terminal, so an
	// interactive shell (zsh -ic) could get SIGTTIN/SIGTTOU and stop meno.
	// Setsid also makes the pid the process group id, so kill(-pid) still works.
	e.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	e.Cancel = func() error {
		if e.Process == nil {
			return nil
		}
		return syscall.Kill(-e.Process.Pid, syscall.SIGKILL)
	}
	e.WaitDelay = time.Second
	limit := s.MaxOut
	if limit <= 0 {
		limit = 64 * 1024
	}
	out, errb := boundedBuffer{max: limit}, boundedBuffer{max: limit}
	e.Stdout = &out
	e.Stderr = &errb
	err := e.Run()
	res := ShResult{Stdout: out.String(), Stderr: errb.String()}
	if c.Err() == context.DeadlineExceeded {
		res.TimedOut = true
	}
	if ee, ok := err.(*exec.ExitError); ok {
		res.ExitCode = ee.ExitCode()
		return res, nil
	}
	return res, err
}

type boundedBuffer struct {
	buf bytes.Buffer
	max int
	cut bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := max(0, b.max-b.buf.Len())
	if n > remaining {
		b.cut = true
	}
	b.buf.Write(p[:min(n, remaining)])
	return n, nil
}
func (b *boundedBuffer) Reset() {
	b.buf.Reset()
	b.cut = false
}

func (b *boundedBuffer) String() string {
	s := b.buf.String()
	if b.cut {
		marker := truncationMarker(b.max)
		return validPrefix(s, b.max-len(marker)) + marker
	}
	return strings.ToValidUTF8(s, "?")
}

// The marker is part of the byte budget. Tiny limits still get a visible signal.
func truncationMarker(limit int) string {
	const marker = "\n…[truncated]"
	if limit >= len(marker) {
		return marker
	}
	return strings.Repeat("!", min(3, max(0, limit)))
}

func validPrefix(s string, limit int) string {
	s = strings.ToValidUTF8(s, "?")
	end := min(len(s), max(0, limit))
	for end > 0 && end < len(s) && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}
