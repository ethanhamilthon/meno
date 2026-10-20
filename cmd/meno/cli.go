package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	menocli "meno/internal/cli"
	"meno/internal/update"
)

const usage = `meno: a terminal coding agent

Usage:
  meno             open the terminal UI
  meno daemon      manage the background daemon
  meno sessions    list sessions
  meno send        write to a session and print the answer
  meno tail        show the events of a session
  meno <group>     agent commands: mcp tasks ask goal web view models read edit write search
  meno update      update meno to the latest GitHub release
  meno -v          print the version
  meno -h          print this help
`

// cli runs a command line other than the bare "meno" and returns the exit code.
func cli(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		return 0
	}
	if isDaemonGroup(args[0]) && !isCLIHelp(args) {
		return runWithDaemon(args, out, errOut)
	}
	if args[0] == "read" || args[0] == "edit" || args[0] == "write" || args[0] == "search" || args[0] == "files" || isCLIHelp(args) {
		return menocli.Run(args, menocli.IO{In: os.Stdin, Out: out, Err: errOut}, nil)
	}
	switch args[0] {
	case "daemon":
		return daemonCLI(args[1:], out, errOut)
	case "-h", "--help":
		return menocli.Run([]string{"--help"}, menocli.IO{In: os.Stdin, Out: out, Err: errOut}, nil)
	case "help":
		return menocli.Run(args, menocli.IO{In: os.Stdin, Out: out, Err: errOut}, nil)
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
