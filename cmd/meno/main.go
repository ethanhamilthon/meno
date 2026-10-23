package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/tui"
	"meno/internal/update"
	ver "meno/internal/version"
)

// version is read from internal/version/VERSION (ver.Number()).
var version = ver.Number()

func main() {
	noAsk := len(os.Args) == 2 && os.Args[1] == "--no-ask"
	if len(os.Args) > 1 && !noAsk {
		os.Exit(cli(os.Args[1:], os.Stdout, os.Stderr))
	}
	if err := run(noAsk); err != nil {
		fmt.Fprintln(os.Stderr, "meno:", err)
		os.Exit(1)
	}
}

func run(noAsk bool) error {
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	// The daemon is started first, before the screen is taken over, so its
	// start-up errors are readable.
	scr, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := scr.Init(); err != nil {
		return err
	}
	defer scr.Fini()
	scr.EnableMouse(tcell.MouseDragEvents) // buttons, wheel and drags: selecting text with the mouse
	cl, err := ensureDaemon(paths.Home)
	if err != nil {
		scr.Fini()
		return err
	}
	var opts []tui.Option
	if noAsk {
		opts = append(opts, tui.WithNoAsk())
	}
	a, err := tui.New(scr, paths, cl, opts...)
	if err != nil {
		return err
	}
	defer a.Close()
	if os.Getenv("MENO_NO_UPDATE_CHECK") == "" {
		c := update.Client{Version: version}
		a.SetUpdateCheck(func(ctx context.Context) string { return c.Check(ctx, paths.Home, version, time.Now()) })
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx)
}
