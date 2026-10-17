package jsvm

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCancelShellAndReuseVM(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	r := v.Exec(ctx, `sh("sleep 30 & wait")`)
	if time.Since(start) > 2*time.Second {
		t.Fatal("shell descendants survived cancellation")
	}
	if r.Err == "" {
		t.Fatal("canceled shell returned success")
	}
	if r = v.Exec(context.Background(), `21*2`); r.Value != "42" || r.Err != "" {
		t.Fatalf("stale interrupt: %+v", r)
	}
}
func TestShellOutputBounded(t *testing.T) {
	shell := Shell{Root: t.TempDir(), MaxOut: 32, Timeout: time.Second}
	r, err := shell.Run(context.Background(), `printf '%01000d' 0`)
	if err != nil || len(r.Stdout) > 64 || !strings.Contains(r.Stdout, "truncated") {
		t.Fatalf("unbounded shell: %+v %v", r, err)
	}
}
func TestAlreadyCanceledDoesNotExecute(t *testing.T) {
	v := newVM(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := v.Exec(ctx, `globalThis.executed=true`); r.Err == "" {
		t.Fatal("canceled execution succeeded")
	}
	if r := v.Exec(context.Background(), `typeof executed`); r.Value != "undefined" {
		t.Fatal("canceled code ran")
	}
}
func TestTimeoutRaceDoesNotPoisonNextExecution(t *testing.T) {
	v := New(Options{Root: t.TempDir(), Timeout: time.Millisecond})
	for i := 0; i < 30; i++ {
		v.Exec(context.Background(), `while(true){}`)
		if r := v.Exec(context.Background(), `1+1`); r.Err != "" || r.Value != "2" {
			t.Fatalf("iteration %d: %+v", i, r)
		}
	}
}
