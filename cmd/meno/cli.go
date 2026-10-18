package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"meno/internal/update"
)

const usage = `meno: a terminal coding agent

Usage:
  meno             open the terminal UI
  meno update      update meno to the latest GitHub release
  meno -v          print the version
  meno -h          print this help
`

// cli runs a command line other than the bare "meno" and returns the exit code.
func cli(args []string, out, errOut io.Writer) int {
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(out, usage)
		return 0
	case "-v", "--version", "version":
		fmt.Fprintln(out, "meno", version)
		return 0
	case "update":
		if len(args) > 1 {
			fmt.Fprintf(errOut, "meno: update takes no arguments\nRun 'meno -h' for usage.\n")
			return 1
		}
		if err := runUpdate(out); err != nil {
			fmt.Fprintln(errOut, "meno: update failed:", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(errOut, "meno: unknown command %q\nRun 'meno -h' for usage.\n", args[0])
	return 1
}

func runUpdate(out io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	c := update.Client{Version: version}
	latest, err := c.Latest(ctx)
	if err != nil {
		return err
	}
	if !update.Newer(version, latest) {
		fmt.Fprintf(out, "meno %s is up to date (latest release: %s)\n", version, latest)
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	fmt.Fprintf(out, "updating meno %s -> %s\n", version, latest)
	if err := c.Install(ctx, latest, exe); err != nil {
		return err
	}
	fmt.Fprintf(out, "updated %s\n", exe)
	return nil
}
