package mcp

import "net/url"

const (
	TransportStdio = "stdio"
	TransportSSE   = "sse"
	TransportHTTP  = "streamable-http"
)

type Server struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	// Headers are sent with every request of an sse or streamable-http server
	// (for example Authorization).
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
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
	switch s.Transport {
	case TransportStdio:
		if _, err := parseCommand(s.Command); err != nil {
			return err
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
