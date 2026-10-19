package jsvm

import (
	"os/exec"
	"runtime"
)

// macProfile lets a command read everything and use the network, but write
// nowhere except the temporary directories and the terminal devices.
const macProfile = `(version 1)(allow default)(deny file-write*)(allow file-write*
 (literal "/dev/null") (literal "/dev/tty") (literal "/dev/dtracehelper")
 (subpath "/private/tmp") (subpath "/private/var/folders") (subpath "/dev/fd"))`

// readOnlyCommand wraps cmd so that it cannot change files outside temporary
// directories. ok is false when this system has no tool for that.
func readOnlyCommand(cmd string) (name string, args []string, ok bool) {
	switch runtime.GOOS {
	case "darwin":
		if p, err := exec.LookPath("sandbox-exec"); err == nil {
			return p, []string{"-p", macProfile, "/bin/sh", "-c", cmd}, true
		}
	case "linux":
		if p, err := exec.LookPath("bwrap"); err == nil {
			return p, []string{"--ro-bind", "/", "/", "--dev", "/dev", "--proc", "/proc", "--tmpfs", "/tmp", "--die-with-parent", "/bin/sh", "-c", cmd}, true
		}
	}
	return "", nil, false
}
