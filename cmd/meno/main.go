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
	l, isLaunch, err := parseLaunch(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "meno:", err)
		os.Exit(2)
	}
	if !isLaunch {
		os.Exit(cli(os.Args[1:], os.Stdout, os.Stderr))
	}
	switch {
	case l.stdio:
		os.Exit(runStdio(l, os.Stdin, os.Stdout, os.Stderr))
	case l.print:
		os.Exit(runHeadless(l, os.Stdin, os.Stdout, os.Stderr))
	}
	if err := run(l); err != nil {
		fmt.Fprintln(os.Stderr, "meno:", err)
		os.Exit(1)
	}
}

func run(l launch) error {
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	if l.cwd != "" {
		paths.Cwd = l.cwd
	}
	// The engine starts before the screen is taken over, so its start-up
	// errors are readable.
	h, err := startHost(paths.Home)
	if err != nil {
		return err
	}
	defer h.Close()
	scr, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := scr.Init(); err != nil {
		return err
	}
	defer scr.Fini()
	scr.EnableMouse(tcell.MouseDragEvents) // buttons, wheel and drags: selecting text with the mouse
	opts := []tui.Option{tui.WithStart(tui.Start{Resume: l.resume, Continue: l.cont, Picker: l.picker})}
	if l.noAsk {
		opts = append(opts, tui.WithNoAsk())
	}
	a, err := tui.New(scr, paths, h.client, opts...)
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
