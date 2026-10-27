package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"meno/internal/config"
	"meno/internal/stdio"
)

// runStdio serves the JSON-RPC protocol of internal/stdio from an engine of
// this process. End of input stops every run and task.
func runStdio(l launch, in io.Reader, out, errOut io.Writer) int {
	paths, err := config.DefaultPaths()
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	h, err := startHost(paths.Home)
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	defer h.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := stdio.Run(ctx, in, out, h.client, stdio.Options{NoAsk: l.noAsk}); err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	return 0
}
