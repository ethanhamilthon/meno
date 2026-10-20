package daemon

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// SocketPath returns the normal path or short hashed fallback for unix limits.
func SocketPath(home string) string {
	path := filepath.Join(home, "run", "meno.sock")
	if len(path) < 100 {
		return path
	}
	sum := sha256.Sum256([]byte(filepath.Clean(home)))
	return filepath.Join(os.TempDir(), fmt.Sprintf("meno-%d-%x.sock", os.Getuid(), sum[:8]))
}
func listenSocket(path string) (net.Listener, error) {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("socket path exists and is not a socket: %s", path)
		}
		if err = os.Remove(path); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}
func closeSocket(path string) {
	if info, e := os.Lstat(path); e == nil && info.Mode()&os.ModeSocket != 0 {
		_ = os.Remove(path)
	}
}
func writePID(run string, pid int) {
	_ = os.WriteFile(filepath.Join(run, "daemon.pid"), []byte(strconv.Itoa(pid)+"\n"), 0o600)
}
func removePID(run string) { _ = os.Remove(filepath.Join(run, "daemon.pid")) }
