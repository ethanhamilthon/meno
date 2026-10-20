package daemon

import (
	"os"
	"path/filepath"
)

// StartupOwnership prevents another daemon from opening services during reset.
// Serve receives the retained handle after the database transition.
func StartupOwnership(home string) (*os.File, error) {
	if os.Getenv("MENO_DAEMON_LOCK_FD") != "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Join(home, "run"), 0700); err != nil {
		return nil, err
	}
	f, err := lockFile(filepath.Join(home, "run"))
	if err != nil {
		return nil, err
	}
	if err = acquireLock(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
