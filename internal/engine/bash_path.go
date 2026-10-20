package engine

import (
	"os"
	"path/filepath"
	"strings"
)

// binDir makes $MENO_HOME/bin/meno point at the running executable, so that
// "meno <group> ..." works inside every bash call, and returns the directory.
// It returns "" when the link cannot be made.
func (e *Engine) binDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	dir := filepath.Join(e.home, "bin")
	link := filepath.Join(dir, "meno")
	if cur, err := os.Readlink(link); err == nil && cur == exe {
		return dir
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	_ = os.Remove(link)
	if err := os.Symlink(exe, link); err != nil {
		return ""
	}
	return dir
}

// withMenoPath returns env with the meno bin directory first in PATH.
func withMenoPath(env []string, dir string) []string {
	if dir == "" {
		return env
	}
	path := os.Getenv("PATH")
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			path = strings.TrimPrefix(kv, "PATH=")
			continue
		}
		out = append(out, kv)
	}
	return append(out, "PATH="+dir+string(os.PathListSeparator)+path)
}
