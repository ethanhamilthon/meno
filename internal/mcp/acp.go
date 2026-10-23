package mcp

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

// ParseACPServers translates the pinned ACP v1 transport union. It never embeds
// client values in errors: commands, URLs and headers may contain credentials.
func ParseACPServers(raw []json.RawMessage, cwd string) ([]Server, error) {
	if len(raw) > 16 {
		return nil, errors.New("at most 16 client MCP servers are supported")
	}
	out := make([]Server, 0, len(raw))
	names := map[string]bool{}
	for _, r := range raw {
		if len(r) > 256<<10 {
			return nil, errors.New("client MCP configuration exceeds size limit")
		}
		var p struct {
			Type    string   `json:"type"`
			Name    string   `json:"name"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
			URL     string   `json:"url"`
			Env     []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"env"`
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		}
		if json.Unmarshal(r, &p) != nil || p.Name == "" || len(p.Name) > 40 || names[p.Name] {
			return nil, errors.New("invalid or duplicate client MCP server")
		}
		names[p.Name] = true
		s := Server{Name: p.Name, Cwd: cwd, Enabled: true, Ephemeral: true, Env: map[string]string{}, Headers: map[string]string{}}
		switch p.Type {
		case "", "stdio":
			if !filepath.IsAbs(p.Command) || strings.ContainsRune(p.Command, 0) || p.Args == nil || p.Env == nil {
				return nil, errors.New("client MCP stdio requires an absolute executable, args and env")
			}
			// Quote the executable for Meno's existing shell-word parser; never invoke a shell.
			s.Transport = TransportStdio
			s.Command = "'" + strings.ReplaceAll(p.Command, "'", "'\"'\"'") + "'"
			s.Args = p.Args
			for _, e := range p.Env {
				if e.Name == "" || strings.ContainsAny(e.Name, "=\x00") || strings.ContainsRune(e.Value, 0) {
					return nil, errors.New("invalid client MCP environment")
				}
				if _, ok := s.Env[e.Name]; ok {
					return nil, errors.New("duplicate client MCP environment variable")
				}
				s.Env[e.Name] = e.Value
			}
		case "http", "sse":
			if p.Headers == nil {
				return nil, errors.New("client MCP remote transport requires headers")
			}
			s.Transport = TransportHTTP
			if p.Type == "sse" {
				s.Transport = TransportSSE
			}
			s.URL = p.URL
			for _, h := range p.Headers {
				k := strings.ToLower(h.Name)
				if k == "" || strings.ContainsAny(k, "\r\n :\t") || strings.ContainsAny(h.Value, "\r\n") {
					return nil, errors.New("invalid client MCP header")
				}
				if _, ok := s.Headers[k]; ok {
					return nil, errors.New("duplicate client MCP header")
				}
				s.Headers[k] = h.Value
			}
		default:
			return nil, errors.New("unsupported client MCP transport")
		}
		if ValidateServer(s) != nil {
			return nil, errors.New("invalid client MCP configuration")
		}
		out = append(out, s)
	}
	return out, nil
}
