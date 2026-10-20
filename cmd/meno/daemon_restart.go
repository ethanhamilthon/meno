package main

import (
	"context"
	"fmt"
	"io"
	"meno/internal/client"
	"time"
)

func restartDaemon(home string, args []string, out, errOut io.Writer) int {
	if !flag(args, "--force") && daemonHasWork(home) {
		fmt.Fprintln(errOut, "meno: daemon has active work; use --force to stop it")
		return 1
	}
	if n := stopDaemon(home, out, errOut); n != 0 {
		return n
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, err := client.New(home).Hello(ctx)
		cancel()
		if err != nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if _, e := client.Ensure(ctx, client.Options{Home: home}); e != nil {
		fmt.Fprintln(errOut, "meno: daemon restart:", e)
		return 1
	}
	fmt.Fprintln(out, "daemon restarted")
	return 0
}
