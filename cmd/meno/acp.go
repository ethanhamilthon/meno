package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"meno/internal/acp"
	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/safe"
)

// runACP serves ACP on stdin/stdout, starting the daemon when it is not running.
func runACP(args []string, in io.Reader, out, errOut io.Writer) int {
	noAsk := len(args) == 1 && args[0] == "--no-ask"
	if len(args) > 0 && !noAsk {
		fmt.Fprintf(errOut, "meno: acp takes only --no-ask\nRun 'meno acp --help' for usage.\n")
		return 2
	}
	paths, err := config.DefaultPaths()
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	c, err := ensureDaemon(paths.Home)
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// An attached ACP connection is a client even before its first session. Reuse
	// the global event stream for presence instead of a polling heartbeat.
	presence, detach := context.WithCancel(ctx)
	presenceDone := make(chan struct{})
	safe.Go(func() { defer close(presenceDone); _ = c.WatchGlobal(presence, func(api.Event) error { return nil }) }, func(err error) { fmt.Fprintln(errOut, "meno: acp:", err) })
	defer func() { detach(); <-presenceDone }()

	var diagnostics io.Writer
	if os.Getenv("MENO_ACP_DIAGNOSTICS") == "1" {
		diagnostics = errOut
	}
	if err := acp.Run(ctx, in, out, c, acp.Options{Version: version, NoAsk: noAsk, Diagnostics: diagnostics}); err != nil {
		fmt.Fprintln(errOut, "meno: acp:", err)
		return 1
	}
	return 0
}
