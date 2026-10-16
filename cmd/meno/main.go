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
)

func main() {
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
	a, err := app.New(scr, paths, llm.NewClient)
	if err != nil {
		return err
	}
	defer a.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx)
}
