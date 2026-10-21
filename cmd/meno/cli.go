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
	if isDaemonGroup(args[0]) && !isCLIHelp(args) {
		return runWithDaemon(args, out, errOut)
	}
	if isCLIHelp(args) {
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
