package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"meno/internal/llm"
	"meno/internal/mcp"
)

// The files below live in the data directory next to meno.db. They only add to
// what the app already has: a name that exists in the app is never replaced,
// and nothing read from a file is ever written to the database.
const (
	ProviderFileName = "provider.json"
	MCPFileName      = "mcp.json"
	ThemeFileName    = "theme.json"
)

func (p Paths) ProviderFile() string { return p.File(ProviderFileName) }
func (p Paths) MCPFile() string      { return p.File(MCPFileName) }
func (p Paths) ThemeFile() string    { return p.File(ThemeFileName) }

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

// LoadProviderFile reads provider.json. An api_key of the form env:NAME is
// replaced by the value of that environment variable. Invalid entries are
// skipped and reported as warnings.
func LoadProviderFile(path string, getenv func(string) string) ([]llm.Provider, []string) {
	raw, warns := ReadObjects(path)
	file := fileName(path)
	var out []llm.Provider
	seen := map[string]bool{}
	for i, r := range raw {
		var p llm.Provider
		if err := json.Unmarshal(r, &p); err != nil {
			warns = append(warns, fmt.Sprintf("%s: entry %d: %v", file, i+1, err))
			continue
		}
		if name, ok := strings.CutPrefix(p.APIKey, "env:"); ok {
			p.APIKey = getenv(strings.TrimSpace(name))
			if p.APIKey == "" {
				warns = append(warns, fmt.Sprintf("%s: %s: environment variable %s is not set", file, p.Name, strings.TrimSpace(name)))
			}
		}
		if err := llm.Validate(p); err != nil {
			warns = append(warns, fmt.Sprintf("%s: %s: %v", file, label(p.Name, i), err))
			continue
		}
		if seen[p.Name] {
			warns = append(warns, fmt.Sprintf("%s: %s: duplicate name, the first one is used", file, p.Name))
			continue
		}
		seen[p.Name] = true
		p.Source = llm.SourceFile
		out = append(out, p)
	}
	return out, warns
}

// LoadMCPFile reads mcp.json: an array of servers in the same shape the app
// uses. A server without "enabled" is enabled.
func LoadMCPFile(path string) ([]mcp.Server, []string) {
	raw, warns := ReadObjects(path)
	file := fileName(path)
	var out []mcp.Server
	seen := map[string]bool{}
	for i, r := range raw {
		var f struct {
			mcp.Server
			Enabled *bool `json:"enabled"`
		}
		if err := json.Unmarshal(r, &f); err != nil {
			warns = append(warns, fmt.Sprintf("%s: entry %d: %v", file, i+1, err))
			continue
		}
		s := f.Server
		s.Enabled = f.Enabled == nil || *f.Enabled
		if s.Transport == "" {
			s.Transport = "stdio"
		}
		if err := mcp.ValidateServer(s); err != nil {
			warns = append(warns, fmt.Sprintf("%s: %s: %v", file, label(s.Name, i), err))
			continue
		}
		if seen[s.Name] {
			warns = append(warns, fmt.Sprintf("%s: %s: duplicate name, the first one is used", file, s.Name))
			continue
		}
		seen[s.Name] = true
		s.Source = mcp.SourceFile
		out = append(out, s)
	}
	return out, warns
}

func label(name string, i int) string {
	if name == "" {
		return fmt.Sprintf("entry %d", i+1)
	}
	return name
}

// MergeProviders appends the file providers to the saved ones. A file provider
// whose name is already saved is dropped: silently when it is identical, with a
// warning when it differs.
func MergeProviders(saved, file []llm.Provider) ([]llm.Provider, []string) {
	out := append([]llm.Provider(nil), saved...)
	var warns []string
	for _, f := range file {
		if old, ok := f.Find(saved); ok {
			f.Source, old.Source = "", ""
			if !reflect.DeepEqual(f, old) {
				warns = append(warns, fmt.Sprintf("%s: %s is ignored: a saved provider with this name exists", ProviderFileName, f.Name))
			}
			continue
		}
		out = append(out, f)
	}
	return out, warns
}

// MergeServers does the same for MCP servers.
func MergeServers(saved, file []mcp.Server) ([]mcp.Server, []string) {
	out := append([]mcp.Server(nil), saved...)
	var warns []string
	for _, f := range file {
		idx := -1
		for i, s := range saved {
			if s.Name == f.Name {
				idx = i
			}
		}
		if idx < 0 {
			out = append(out, f)
			continue
		}
		a, b := f, saved[idx]
		a.Source, b.Source = "", ""
		if !reflect.DeepEqual(a, b) {
			warns = append(warns, fmt.Sprintf("%s: %s is ignored: a saved server with this name exists", MCPFileName, f.Name))
		}
	}
	return out, warns
}

// OwnProviders drops the providers that came from a file: only the rest is saved.
func OwnProviders(ps []llm.Provider) []llm.Provider {
	var out []llm.Provider
	for _, p := range ps {
		if p.Source != llm.SourceFile && p.Source != llm.SourceOAuth {
			out = append(out, p)
		}
	}
	return out
}

// OwnServers drops the servers that came from a file.
func OwnServers(sv []mcp.Server) []mcp.Server {
	var out []mcp.Server
	for _, s := range sv {
		if s.Source != mcp.SourceFile {
			out = append(out, s)
		}
	}
	return out
}
