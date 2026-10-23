package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Providers, MCP servers and themes live in JSON files in the data directory.
// Each file is a JSON array of objects; the app reads and writes provider.json
// and mcp.json, theme.json is only read.
const (
	ProviderFileName = "provider.json"
	MCPFileName      = "mcp.json"
	ThemeFileName    = "theme.json"
)

func (p Paths) ProviderFile() string { return p.File(ProviderFileName) }
func (p Paths) MCPFile() string      { return p.File(MCPFileName) }
func (p Paths) ThemeFile() string    { return p.File(ThemeFileName) }

// listMu serializes every read-modify-write of provider.json and mcp.json in
// this process. The daemon is the only writer.
var listMu sync.Mutex

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

// ReadObjects decodes the array of objects in path into raw entries; it is the
// shared reader for the theme file. Problems are returned as warnings.
func ReadObjects(path string) ([]json.RawMessage, []string) {
	raw, err := readArray(path)
	if err != nil {
		return nil, []string{fileName(path) + ": " + err.Error()}
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
