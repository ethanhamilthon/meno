package main

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// oldDaemonSocket is where meno before 0.8.0 listened: run/meno.sock, or a short
// hashed path in the temp directory when that one is too long for a unix socket.
func oldDaemonSocket(home string) string {
	path := filepath.Join(home, "run", "meno.sock")
	if len(path) < 100 {
		return path
	}
	sum := sha256.Sum256([]byte(filepath.Clean(home)))
	return filepath.Join(os.TempDir(), fmt.Sprintf("meno-%d-%x.sock", os.Getuid(), sum[:8]))
}

// checkOldDaemon refuses to start while a daemon of meno 0.7 still runs for this
// MENO_HOME: it owns the sessions that are running and a second engine would
// race with it. The daemon exits by itself 10 seconds after its last window closes.
func checkOldDaemon(home string) error {
	sock := oldDaemonSocket(home)
	if _, err := os.Stat(sock); err != nil {
		return nil
	}
	c, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		return nil // a stale file
	}
	c.Close()
	return fmt.Errorf("a background daemon of an older meno is still running for %s. Close every meno window of that version; the daemon exits about 10 seconds later. Then start meno again", home)
}
