package tools

import "os/exec"

// RunRestricted starts the daemon executable with parsed argv and no shell. It
// returns the running process handle so callers can stream output or cancel it.
func RunRestricted(command, executable string, allowedGroups []string, options RunOptions) (*Call, error) {
	parsed, err := ParseRestrictedCommand(command, executable, allowedGroups)
	if err != nil {
		return nil, err
	}
	return StartProgram(options, parsed.Path, parsed.Args)
}

// StartProgram starts a program with an argv and no shell.
func StartProgram(options RunOptions, path string, args []string) (*Call, error) {
	return start(options, exec.Command(path, args...))
}
