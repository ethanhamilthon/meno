package main

import (
	"fmt"
	"io"
	"os"

	menocli "meno/internal/cli"
)

// cli runs a command line other than the bare "meno" and returns the exit code.
func cli(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		return 0
	}
	if isCLIHelp(args) {
		return menocli.Run(args, menocli.IO{In: os.Stdin, Out: out, Err: errOut}, nil)
	}
	switch args[0] {
	case "mcp", "tasks", "ask", "view", "models", "doctor":
		return runCommand(args, out, errOut)
	case "daemon":
		fmt.Fprintln(errOut, "meno: the daemon was removed in 0.8.0; meno runs its engine in each process. See docs/upgrading.md")
		return 2
	case "send", "tail", "sessions":
		fmt.Fprintf(errOut, "meno: %s was removed in 0.8.0; use meno -p \"...\" (and --resume, --continue). See docs/upgrading.md\n", args[0])
		return 2
	case "-h", "--help":
		return menocli.Run([]string{"--help"}, menocli.IO{In: os.Stdin, Out: out, Err: errOut}, nil)
	case "help":
		return menocli.Run(args, menocli.IO{In: os.Stdin, Out: out, Err: errOut}, nil)
	case "-v", "--version", "version":
		fmt.Fprintln(out, "meno", version)
		return 0
	case "acp":
		return runACP(args[1:], os.Stdin, out, errOut)
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

// runCommand runs a command of the cli package. Inside an agent call it talks
// to the owner of the call; a human command starts an engine for its own run.
func runCommand(args []string, out, errOut io.Writer) int {
	streams := menocli.IO{In: os.Stdin, Out: out, Err: errOut}
	if menocli.InsideCall() {
		return menocli.Run(args, streams, menocli.FromEnv())
	}
	switch args[0] {
	case "tasks", "ask", "view":
		fmt.Fprintf(errOut, "meno: %s works inside an agent call (the agent runs it)\n", args[0])
		return 1
	}
	h, err := startHost(menocli.Home())
	if err != nil {
		fmt.Fprintln(errOut, "meno:", err)
		return 1
	}
	defer h.Close()
	return menocli.Run(args, streams, menocli.New(menocli.Home(), h.client, "", "", ""))
}
