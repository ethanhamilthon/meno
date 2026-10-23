package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"meno/internal/acp"
	"meno/internal/config"
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
	if err := acp.Run(ctx, in, out, c, acp.Options{Version: version, NoAsk: noAsk}); err != nil {
		fmt.Fprintln(errOut, "meno: acp:", err)
		return 1
	}
	return 0
}
