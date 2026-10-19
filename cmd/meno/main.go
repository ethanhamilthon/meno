package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/app"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/update"
	ver "meno/internal/version"
)

// version defaults to internal/version/VERSION; a release may override it with
// -ldflags "-X main.version=…".
var version = ver.Number()

func main() {
	if len(os.Args) > 1 {
		os.Exit(cli(os.Args[1:], os.Stdout, os.Stderr))
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "meno:", err)
		os.Exit(1)
	}
}

func run() error {
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	scr, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := scr.Init(); err != nil {
		return err
	}
	defer scr.Fini()
	scr.EnableMouse(tcell.MouseDragEvents) // buttons, wheel and drags: selecting text with the mouse
	a, err := app.New(scr, paths, llm.NewClient)
	if err != nil {
		return err
	}
	defer a.Close()
	if os.Getenv("MENO_NO_UPDATE_CHECK") == "" {
		c := update.Client{Version: version}
		a.SetUpdateCheck(func(ctx context.Context) string { return c.Check(ctx, paths.Home, version, time.Now()) })
		a.EnableOAuthUpdates()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx)
}
