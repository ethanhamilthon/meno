package mcp

import (
	"net/url"
	"strings"
)

const (
	TransportStdio = "stdio"
	TransportSSE   = "sse"
	TransportHTTP  = "streamable-http"
)

type Server struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`
	Command   string `json:"command"`
	// Args are the arguments of a stdio server, appended to the words of
	// Command. The usual MCP shape ("command": "python", "args": ["s.py"])
	// works; a Command that already names the script keeps working.
	Args []string `json:"args,omitempty"`
	// Cwd is the working directory of a stdio server. Empty means the
	// working directory of the daemon.
	Cwd string            `json:"cwd,omitempty"`
	Env map[string]string `json:"env,omitempty"`
	URL string            `json:"url,omitempty"`
	// Headers are sent with every request of an sse or streamable-http server
	// (for example Authorization).
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
	// Protocol picks the protocol era: "auto" or empty (probe the modern
	// 2026-07-28 era first and fall back to legacy), "legacy" (initialize
	// handshake only, 2024-11-05 to 2025-06-18) or "modern". sse servers are
	// always legacy.
	Protocol string `json:"protocol,omitempty"`
}

func ValidateServer(s Server) error {
	if s.Name == "" || len(s.Name) > 40 {
		return errString("name must be 1-40 characters")
	}
	for _, r := range s.Name {
		if !(r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return errString("name may contain only a-z, 0-9, - and _")
		}
	}
	switch s.Protocol {
	case "", ProtocolLegacy, ProtocolAuto:
	case ProtocolModern:
		if s.Transport == TransportSSE {
			return errString("the sse transport cannot use the modern protocol")
		}
	default:
		return errString("protocol must be legacy, auto or modern")
	}
	switch s.Transport {
	case TransportStdio:
		if _, err := parseCommand(s.Command); err != nil {
			return err
		}
		for _, a := range s.Args {
			if strings.TrimSpace(a) == "" {
				return errString("args must not be empty")
			}
		}
	case TransportSSE, TransportHTTP:
		if s.URL == "" {
			return errString("url is required for " + s.Transport)
		}
		if u, err := url.Parse(s.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errString("url must be a full http(s) address")
		}
	default:
		return errString("transport must be stdio, sse or streamable-http")
	}
	return nil
}

type errString string

func (e errString) Error() string { return string(e) }
