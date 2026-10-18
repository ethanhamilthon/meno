package jsvm

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestShellCommandHasNoControllingTerminal(t *testing.T) {
	sh := Shell{Root: t.TempDir(), Timeout: 10 * time.Second}
	res, err := sh.Run(context.Background(), "if true </dev/tty 2>/dev/null; then echo tty; else echo notty; fi")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(res.Stdout); got != "notty" {
		t.Fatalf("command still has a controlling terminal: %q", got)
	}
}

func TestShellTimeoutKillsProcessGroup(t *testing.T) {
	sh := Shell{Root: t.TempDir(), Timeout: 300 * time.Millisecond}
	start := time.Now()
	res, err := sh.Run(context.Background(), "sleep 30 & sleep 30")
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut {
		t.Fatalf("expected timeout, got %+v", res)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("timeout did not stop the group quickly: %s", d)
	}
}
