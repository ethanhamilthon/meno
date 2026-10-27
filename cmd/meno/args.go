package main

import (
	"errors"
	"strings"
)

// launch is a command line that starts a session: the terminal UI, a headless
// run (-p) or the JSON-RPC server (-p --stdio). Anything else is a meno
// command (mcp, tasks, doctor...) and goes to the cli package.
type launch struct {
	print    bool   // -p: no terminal UI
	stdio    bool   // --stdio: serve JSON-RPC on stdin/stdout
	json     bool   // --json: print the whole result as JSON
	noSave   bool   // --no-save: the session is not stored
	noAsk    bool   // --no-ask: the agent cannot ask the user
	cont     bool   // --continue: the newest session of the directory
	picker   bool   // resume without an ID: open the list of sessions
	resume   string // session to continue
	model    string
	cwd      string
	prompt   string
	hasInput bool // a prompt was given on the command line
}

var errUsage = errors.New("usage: meno [-p [PROMPT]] [--stdio] [--resume ID | --continue] [--model P/M] [--cwd DIR] [--no-ask] [--no-save] [--json]  |  meno resume [ID]")

// parseLaunch reports whether args start a session and, if so, what to start.
func parseLaunch(args []string) (l launch, ok bool, err error) {
	if len(args) == 0 {
		return l, true, nil
	}
	first := args[0]
	if first != "resume" && !isLaunchFlag(first) {
		return l, false, nil
	}
	var words []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		value := func() (string, error) {
			if i+1 >= len(args) {
				return "", errors.New(a + " needs a value")
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "--":
			words = append(words, args[i+1:]...)
			i = len(args)
		case a == "resume" && i == 0:
			l.picker = true
		case a == "-p" || a == "--print":
			l.print = true
		case a == "--stdio":
			l.stdio = true
		case a == "--json":
			l.json = true
		case a == "--no-save":
			l.noSave = true
		case a == "--no-ask":
			l.noAsk = true
		case a == "--continue" || a == "-c":
			l.cont = true
		case a == "--resume":
			if l.resume, err = value(); err != nil {
				return l, true, err
			}
		case a == "--model":
			if l.model, err = value(); err != nil {
				return l, true, err
			}
		case a == "--cwd":
			if l.cwd, err = value(); err != nil {
				return l, true, err
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			return l, true, errors.New("unknown option " + a + "\n" + errUsage.Error())
		default:
			words = append(words, a)
		}
	}
	if l.picker {
		if len(words) > 1 {
			return l, true, errUsage
		}
		if len(words) == 1 {
			l.resume, l.picker = words[0], false
		}
		words = nil
	}
	l.prompt = strings.Join(words, " ")
	l.hasInput = len(words) > 0
	switch {
	case l.resume != "" && l.cont:
		return l, true, errors.New("--resume and --continue cannot be combined")
	case l.noSave && (l.resume != "" || l.cont):
		return l, true, errors.New("--no-save cannot be combined with --resume or --continue: a stored session is saved")
	case l.stdio && !l.print:
		return l, true, errors.New("--stdio needs -p")
	case l.stdio && (l.hasInput || l.json || l.noSave):
		return l, true, errors.New("--stdio takes no prompt, --json or --no-save")
	case !l.print && (l.json || l.noSave || l.hasInput || l.model != ""):
		return l, true, errors.New("a prompt, --json, --no-save and --model need -p")
	}
	return l, true, nil
}

func isLaunchFlag(a string) bool {
	switch a {
	case "-p", "--print", "--stdio", "--json", "--no-save", "--no-ask", "--continue", "-c", "--resume", "--model", "--cwd":
		return true
	}
	return false
}
