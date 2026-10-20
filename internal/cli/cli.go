package cli

import (
	"io"
	"os"
	"strings"
)

const (
	// ExitOK means the command succeeded.
	ExitOK = 0
	// ExitError means a command or daemon operation failed.
	ExitError = 1
	// ExitUsage means command syntax is invalid.
	ExitUsage = 2
	// ExitForbidden means the active mode disallows a command.
	ExitForbidden = 3
	// ExitWaitTimeout means a wait timed out without cancelling its work.
	ExitWaitTimeout = 124
)

// IO contains the streams and working directory used by a CLI invocation.
type IO struct {
	In       io.Reader
	Out, Err io.Writer
	Dir      string
}

// Run parses and executes one meno command. Daemon-backed commands use api.
func Run(args []string, streams IO, api API) int {
	if streams.In == nil {
		streams.In = strings.NewReader("")
	}
	if streams.Out == nil {
		streams.Out = io.Discard
	}
	if streams.Err == nil {
		streams.Err = io.Discard
	}
	if streams.Dir == "" {
		streams.Dir, _ = os.Getwd()
	}
	if len(args) == 0 {
		return fail(streams, usageError("expected a command"))
	}
	if args[0] == "--help" || args[0] == "-h" {
		return printHelp(streams, "")
	}
	if args[0] == "help" {
		if len(args) > 2 {
			return fail(streams, usageError("usage: meno help [TOPIC]"))
		}
		if len(args) == 2 {
			return printHelp(streams, args[1])
		}
		return printHelp(streams, "")
	}
	if args[0] == "meno" {
		if len(args) == 1 {
			return printHelp(streams, "")
		}
		if args[1] == "--help" || args[1] == "-h" {
			return printHelp(streams, "")
		}
		args = args[1:]
	}
	if len(args) == 0 {
		return fail(streams, usageError("expected a command"))
	}
	name := args[0]
	args = args[1:]
	jsonOut := false
	for i := 0; i < len(args); {
		if args[i] == "--json" {
			jsonOut = true
			args = append(args[:i], args[i+1:]...)
			continue
		}
		i++
	}
	for i, arg := range args {
		if arg == "--help" || arg == "-h" {
			return printHelp(streams, helpTopic(name, args[:i]))
		}
	}
	var code int
	var err error
	switch name {
	case "mcp":
		if len(args) > 0 && !containsString([]string{"servers", "tools", "call", "results", "add", "remove", "restart"}, args[0]) {
			code, err = runMCP(append([]string{"call"}, args...), streams, api, jsonOut)
		} else {
			code, err = runMCP(args, streams, api, jsonOut)
		}
	case "tasks":
		code, err = runTasks(args, streams, api, jsonOut)
	case "ask":
		code, err = runAsk(args, streams, api, jsonOut)
	case "view":
		code, err = runView(args, streams, api, jsonOut)
	case "models":
		code, err = runModels(args, streams, api, jsonOut)
	case "daemon":
		code, err = runDaemon(args, streams, api, jsonOut)
	case "sessions":
		code, err = runSessions(args, streams, api, jsonOut)
	case "send":
		code, err = runSend(args, streams, api, jsonOut)
	case "tail":
		code, err = runTail(args, streams, api, jsonOut)
	default:
		err = unknown(name, append(commandNames, "help"))
	}
	if err != nil {
		return fail(streams, err)
	}
	return code
}
