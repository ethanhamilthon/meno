package agent

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// shellTool is a small ToolRunner for tests: each call runs /bin/sh -c and the
// output is what the shell printed. It stands in for the engine's bash runner.
type shellTool struct {
	dir     string
	timeout time.Duration
	calls   int
}

func newShellTool(dir string, timeout time.Duration) *shellTool {
	return &shellTool{dir: dir, timeout: timeout}
}

func (s *shellTool) Timeout() time.Duration { return s.timeout }

func (s *shellTool) Run(ctx context.Context, code string, limit time.Duration) (string, string, string, bool, time.Duration) {
	s.calls++
	start := time.Now()
	if limit > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, limit)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", code)
	cmd.Dir = s.dir
	out, err := cmd.CombinedOutput()
	text := strings.TrimRight(string(out), "\n")
	if ctx.Err() == context.DeadlineExceeded {
		return text, "", "timeout after " + limit.String(), true, time.Since(start)
	}
	if err != nil {
		if ctx.Err() != nil {
			return text, "", "execution canceled", false, time.Since(start)
		}
		return text, "", "", false, time.Since(start)
	}
	return text, "", "", false, time.Since(start)
}
