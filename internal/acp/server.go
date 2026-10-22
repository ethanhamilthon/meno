package acp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"sync"
	"time"

	"meno/internal/api"
)

// ProtocolVersion is the ACP version spoken.
const ProtocolVersion = 1

// Options configures a Server.
type Options struct {
	Version string
}

// Server is one ACP connection to one client.
type Server struct {
	backend Backend
	opts    Options
	conn    *conn
	ctx     context.Context

	mu       sync.Mutex
	sessions map[string]*session
	models   []api.ConfigModel
}

// Run serves ACP on in and out until in ends or ctx is done. Diagnostics must
// go to stderr by the caller: out carries only protocol messages.
func Run(ctx context.Context, in io.Reader, out io.Writer, b Backend, o Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &Server{backend: b, opts: o, conn: newConn(out), ctx: ctx, sessions: map[string]*session{}}
	return s.conn.serve(in, s.methods(), func() {
		s.shutdown()
		cancel()
	})
}

// shutdown detaches every session: executing runs stop, paused questions stay.
func (s *Server) shutdown() {
	s.mu.Lock()
	all := make([]*session, 0, len(s.sessions))
	for _, x := range s.sessions {
		all = append(all, x)
	}
	s.mu.Unlock()
	for _, x := range all {
		x.detach()
	}
}

func (s *Server) methods() map[string]handler {
	return map[string]handler{
		"initialize":                s.initialize,
		"session/new":               s.newSession,
		"session/load":              s.loadSession,
		"session/list":              s.listSessions,
		"session/prompt":            s.prompt,
		"session/cancel":            s.cancel,
		"session/set_config_option": s.setConfig,
	}
}

func (s *Server) initialize(params json.RawMessage) (any, error) {
	return map[string]any{
		"protocolVersion": ProtocolVersion,
		"agentInfo":       map[string]any{"name": "meno", "title": "Meno", "version": s.opts.Version},
		"authMethods":     []any{},
		"agentCapabilities": map[string]any{
			"loadSession":         true,
			"promptCapabilities":  map[string]any{"image": true, "audio": false, "embeddedContext": true},
			"mcpCapabilities":     map[string]any{"http": false, "sse": false},
			"sessionCapabilities": map[string]any{"list": map[string]any{}},
		},
	}, nil
}

type newParams struct {
	Cwd        string            `json:"cwd"`
	SessionID  string            `json:"sessionId"`
	MCPServers []json.RawMessage `json:"mcpServers"`
}

func (p newParams) check() error {
	if p.Cwd == "" {
		return errParams("cwd is required")
	}
	if len(p.MCPServers) > 0 {
		return errParams("mcpServers are not supported: configure MCP servers in Meno with `meno mcp`")
	}
	return nil
}

func clientID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (s *Server) newSession(raw json.RawMessage) (any, error) {
	var p newParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errParams("invalid params: %v", err)
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	res, err := s.backend.CreateSession(s.ctx, api.SessionCreateRequest{Cwd: p.Cwd, ClientID: clientID()})
	if err != nil {
		return nil, err
	}
	x, err := s.attach(res.Session.ID, false)
	if err != nil {
		return nil, err
	}
	out := s.sessionResult(x)
	out["sessionId"] = x.id
	s.afterSessionStart(x)
	return out, nil
}

func (s *Server) loadSession(raw json.RawMessage) (any, error) {
	var p newParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errParams("invalid params: %v", err)
	}
	if p.SessionID == "" {
		return nil, errParams("sessionId is required")
	}
	if len(p.MCPServers) > 0 {
		return nil, errParams("mcpServers are not supported: configure MCP servers in Meno with `meno mcp`")
	}
	snap, err := s.backend.Snapshot(s.ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	if p.Cwd != "" && snap.Cwd != "" && p.Cwd != snap.Cwd {
		return nil, errParams("session %s belongs to %s, not %s", p.SessionID, snap.Cwd, p.Cwd)
	}
	s.detachExisting(p.SessionID)
	x, err := s.attach(p.SessionID, true)
	if err != nil {
		return nil, err
	}
	s.afterSessionStart(x)
	return s.sessionResult(x), nil
}

func (s *Server) detachExisting(id string) {
	s.mu.Lock()
	old := s.sessions[id]
	delete(s.sessions, id)
	s.mu.Unlock()
	if old != nil && old.stop != nil {
		old.stop()
	}
}

func (s *Server) attach(id string, loading bool) (*session, error) {
	x := newSession(id, s, loading)
	s.loadModels()
	s.mu.Lock()
	s.sessions[id] = x
	s.mu.Unlock()
	if err := x.follow(s.ctx); err != nil {
		s.mu.Lock()
		delete(s.sessions, id)
		s.mu.Unlock()
		return nil, err
	}
	x.mu.Lock()
	x.ctxSize = s.contextWindow(x.prov, x.model)
	x.mu.Unlock()
	return x, nil
}

// afterSessionStart publishes the command list once the response is on its way.
func (s *Server) afterSessionStart(x *session) {
	go func() {
		time.Sleep(20 * time.Millisecond)
		s.sendCommands(x)
	}()
}

func (s *Server) listSessions(raw json.RawMessage) (any, error) {
	var p struct {
		Cwd string `json:"cwd"`
	}
	_ = json.Unmarshal(raw, &p)
	res, err := s.backend.ListSessions(s.ctx, api.SessionsListRequest{Cwd: p.Cwd, Limit: 200})
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(res.Sessions))
	for _, it := range res.Sessions {
		m := map[string]any{"sessionId": it.ID, "cwd": it.Cwd}
		if it.Name != "" {
			m["title"] = it.Name
		}
		if it.UpdatedAt != "" {
			m["updatedAt"] = it.UpdatedAt
		}
		items = append(items, m)
	}
	return map[string]any{"sessions": items}, nil
}

func (s *Server) session(id string) (*session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.sessions[id]
	if x == nil {
		return nil, errParams("unknown session %q: call session/new or session/load first", id)
	}
	return x, nil
}

func (s *Server) cancel(raw json.RawMessage) (any, error) {
	var p struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return nil, nil
	}
	if x, err := s.session(p.SessionID); err == nil {
		x.cancel(s.ctx)
	}
	return nil, nil
}

// modelValue encodes a provider and model into one reversible config value.
func modelValue(provider, model string) string {
	return url.PathEscape(provider) + "/" + url.PathEscape(model)
}

func parseModelValue(v string) (provider, model string, ok bool) {
	for i := 0; i < len(v); i++ {
		if v[i] == '/' {
			p, e1 := url.PathUnescape(v[:i])
			m, e2 := url.PathUnescape(v[i+1:])
			return p, m, e1 == nil && e2 == nil && p != "" && m != ""
		}
	}
	return "", "", false
}

// uncancelled is the context of a message submission. Closing the connection
// must not abandon a submission the daemon may already have accepted: the
// request id then arrives late and the run is stopped by session.started.
func (s *Server) uncancelled() context.Context { return context.WithoutCancel(s.ctx) }
