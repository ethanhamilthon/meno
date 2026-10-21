package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/update"
)

type stagedUpdate interface {
	Commit(context.Context) error
	Close() error
}

type updateDependencies struct {
	latest  func(context.Context) (string, error)
	prepare func(context.Context, string, string) (stagedUpdate, error)
	lock    func(context.Context) (io.Closer, error)
	stop    func(context.Context) (io.Closer, error)
}

func runUpdate(out io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	c := update.Client{Version: version}
	return performUpdate(ctx, out, version, exe, updateDependencies{
		latest:  c.Latest,
		prepare: func(ctx context.Context, tag, exe string) (stagedUpdate, error) { return c.Prepare(ctx, tag, exe) },
		lock:    func(ctx context.Context) (io.Closer, error) { return daemon.AcquireTransition(ctx, paths.Home) },
		stop: func(ctx context.Context) (io.Closer, error) {
			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			return client.StopConfirmed(ctx, paths.Home, true)
		},
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
	// Serialize preparation too: competing updaters must never commit stale work
	// or replace an executable that changed while they were waiting.
	lock, err := d.lock(ctx)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := daemon.CheckExecutable(); err != nil {
		return err
	}
	fmt.Fprintf(out, "updating meno %s -> %s\n", current, latest)
	staged, err := d.prepare(ctx, latest, exe)
	if err != nil {
		return err
	}
	defer staged.Close()
	fmt.Fprintln(out, "new executable verified and staged")
	ownership, err := d.stop(ctx)
	if err != nil {
		return err
	}
	defer ownership.Close()
	fmt.Fprintln(out, "daemon stopped; cleanup confirmed")
	if err := staged.Commit(ctx); err != nil {
		return fmt.Errorf("executable replacement failed: %w; old executable retained, daemon remains stopped; reopen meno to start it again", err)
	}
	fmt.Fprintf(out, "updated %s\nClose and reopen meno to start the new daemon.\n", exe)
	return nil
}
