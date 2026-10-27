package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"meno/internal/update"
)

type stagedUpdate interface {
	Commit(context.Context) error
	Close() error
}

type updateDependencies struct {
	latest  func(context.Context) (string, error)
	prepare func(context.Context, string, string) (stagedUpdate, error)
}

func runUpdate(out io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	c := update.Client{Version: version}
	return performUpdate(ctx, out, version, exe, updateDependencies{
		latest:  c.Latest,
		prepare: func(ctx context.Context, tag, exe string) (stagedUpdate, error) { return c.Prepare(ctx, tag, exe) },
	})
}

func performUpdate(ctx context.Context, out io.Writer, current, exe string, d updateDependencies) error {
	latest, err := d.latest(ctx)
	if err != nil {
		return err
	}
	if !update.Newer(current, latest) {
		fmt.Fprintf(out, "meno %s is up to date (latest release: %s)\n", current, latest)
		return nil
	}
	fmt.Fprintf(out, "updating meno %s -> %s\n", current, latest)
	staged, err := d.prepare(ctx, latest, exe)
	if err != nil {
		return err
	}
	defer staged.Close()
	fmt.Fprintln(out, "new executable verified and staged")
	if err := staged.Commit(ctx); err != nil {
		return fmt.Errorf("executable replacement failed: %w; the old executable is kept", err)
	}
	fmt.Fprintf(out, "updated %s\nReopen meno to use it.\n", exe)
	return nil
}
