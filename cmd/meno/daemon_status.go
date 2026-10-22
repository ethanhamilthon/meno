package main

import (
	"context"
	"fmt"
	"io"
	"meno/internal/api"
	"meno/internal/client"
	"time"
)

func daemonStatus(home string, out io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	h, e := client.New(home).Hello(ctx)
	if e != nil {
		fmt.Fprintln(out, "daemon stopped")
		return 0
	}
	fmt.Fprintf(out, "daemon running (pid %d, protocol %d, boot %s)\n", h.PID, h.Protocol, h.BootID)
	return 0
}
func daemonHasWork(home string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, e := client.New(home).ListSessions(ctx, api.SessionsListRequest{})
	if e != nil {
		return false
	}
	for _, s := range r.Sessions {
		if s.Running {
			return true
		}
	}
	return false
}
func flag(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
