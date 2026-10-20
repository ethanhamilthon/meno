package main

import (
	"context"
	"fmt"
	"io"
	"meno/internal/client"
	"time"
)

func stopDaemon(home string, out, errOut io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if e := client.New(home).Shutdown(ctx, "client request"); e != nil {
		fmt.Fprintln(out, "daemon stopped")
		return 0
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		probe, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, e := client.New(home).Hello(probe)
		stop()
		if e != nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	fmt.Fprintln(out, "daemon stopped")
	return 0
}
