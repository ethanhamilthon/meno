package mcp

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
	Enabled   bool              `json:"enabled"`
}

func DefaultServers() []Server {
	return []Server{
		{Name: "postgres", Transport: TransportStdio, Command: "npx -y @mcp/postgres", Env: map[string]string{"PG_URL": "postgres://localhost:5432/app"}, Enabled: true},
		{Name: "github", Transport: TransportStdio, Command: "gh-mcp-server", Enabled: true},
		{Name: "sentry", Transport: TransportSSE, URL: "http://localhost:9100", Enabled: true},
		{Name: "playwright", Transport: TransportStdio, Command: "npx -y @mcp/playwright", Enabled: false},
	}
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
		if s.Command == "" {
			return errString("command is required for stdio")
		}
	case TransportSSE, TransportHTTP:
		if s.URL == "" {
			return errString("url is required for " + s.Transport)
		}
	default:
		return errString("transport must be stdio, sse or streamable-http")
	}
	return nil
}

type errString string

func (e errString) Error() string { return string(e) }
