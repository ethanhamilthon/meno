package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

// Providers and MCP servers use arrays of objects in the data directory.
// The single palette object is handled separately in theme_file.go.
const (
	ProviderFileName = "provider.json"
	MCPFileName      = "mcp.json"
	ThemeFileName    = "theme.json"
)

func (p Paths) ProviderFile() string { return p.File(ProviderFileName) }
func (p Paths) MCPFile() string      { return p.File(MCPFileName) }
func (p Paths) ThemeFile() string    { return p.File(ThemeFileName) }

// listMu serializes reads and writes of provider.json and mcp.json in this
// process. A read-modify-write that spans several calls takes LockFiles first.
var listMu sync.Mutex

var filesMu sync.Mutex

// LockFiles serializes read-modify-write of meno's JSON files (settings,
// providers, MCP servers) across the processes of one MENO_HOME: an exclusive
// flock on run/config.lock, which the system drops if the process dies. Keep
// the critical section short. Call the returned function to release it.
func (p Paths) LockFiles() (func(), error) {
	filesMu.Lock()
	dir := filepath.Join(p.Home, "run")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		filesMu.Unlock()
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "config.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		filesMu.Unlock()
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		filesMu.Unlock()
		return nil, err
	}
	return func() { f.Close(); filesMu.Unlock() }, nil
}

// readArray decodes a JSON array of objects. A missing or empty file is not an
// error and gives no entries.
func readArray(path string) ([]json.RawMessage, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || (err == nil && strings.TrimSpace(string(b)) == "") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("expected a JSON array of objects: %w", err)
	}
	return raw, nil
}

// readFile reads a list file for the app. A file that is not a JSON array is an
// error that names the file.
func readFile(path string) ([]json.RawMessage, error) {
	raw, err := readArray(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fileName(path), err)
	}
	return raw, nil
}

// readForWrite is readFile with a message that says why nothing was saved: a
// file the user broke by hand is never overwritten.
func readForWrite(path string) ([]json.RawMessage, error) {
	raw, err := readArray(path)
	if err != nil {
		return nil, fmt.Errorf("%s is not valid (%v); fix or remove the file, meno does not overwrite it", fileName(path), err)
	}
	return raw, nil
}

func fileName(path string) string {
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func label(name string, i int) string {
	if name == "" {
		return fmt.Sprintf("entry %d", i+1)
	}
	return name
}
