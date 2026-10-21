package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"meno/internal/client"
	"meno/internal/daemon"
)

func stopDaemon(home string, out, errOut io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	transition, err := daemon.AcquireTransition(ctx, home)
	if err == nil {
		defer transition.Close()
		var ownership io.Closer
		ownership, err = client.StopConfirmed(ctx, home, false)
		if err == nil {
			ownership.Close()
		}
	}
	if err != nil {
		fmt.Fprintln(errOut, "meno: daemon stop:", err)
		return 1
	}
	fmt.Fprintln(out, "daemon stopped")
	return 0
}
