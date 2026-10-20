package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	menocli "meno/internal/cli"
	"meno/internal/cli/daemonapi"
	"meno/internal/client"
)

// isDaemonGroup reports the commands that talk to the daemon.
func isDaemonGroup(name string) bool {
	switch name {
	case "mcp", "tasks", "ask", "goal", "view", "models", "sessions", "send", "tail":
		return true
	}
	return false
}

// runWithDaemon runs a daemon-backed command. Inside an agent call the daemon
// is already running and the environment names the session; a human command
// starts the daemon when it is not up.
func runWithDaemon(args []string, out, errOut io.Writer) int {
	api, err := daemonapi.FromEnv()
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	if os.Getenv("MENO_TOKEN") == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if _, err := client.Ensure(ctx, client.Options{Home: api.Home()}); err != nil {
			fmt.Fprintln(errOut, "meno: daemon:", err)
			return 1
		}
	}
	return menocli.Run(args, menocli.IO{In: os.Stdin, Out: out, Err: errOut}, api)
}
