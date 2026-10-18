package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gdamore/tcell/v2"

	"meno/internal/app"
	"meno/internal/config"
	"meno/internal/llm"
	ver "meno/internal/version"
)

// version defaults to internal/version/VERSION; a release may override it with
// -ldflags "-X main.version=…".
var version = ver.Number()

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println("meno", version)
		return
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx)
}
