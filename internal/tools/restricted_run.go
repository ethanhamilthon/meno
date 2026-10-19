package tools

import (
	"context"
	"os/exec"
)

// RunRestricted starts the daemon executable with parsed argv and no shell. It
// returns the running process handle so callers can stream output or cancel it.
func RunRestricted(ctx context.Context, command, executable string, allowedGroups []string, options RunOptions) (*Call, error) {
	parsed, err := ParseRestrictedCommand(command, executable, allowedGroups)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(parsed.Path, parsed.Args...)
	return start(ctx, options, cmd)
}
