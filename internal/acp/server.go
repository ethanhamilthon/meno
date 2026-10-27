package acp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"path/filepath"
	"sync"

	"meno/internal/api"
)

// ProtocolVersion is the ACP version spoken.
const ProtocolVersion = 1

// Options configures a Server.
type Options struct {
	Version string
	// NoAsk creates every session with the ask tool turned off.
	NoAsk bool
	// Diagnostics optionally receives metadata-only JSON records; nil disables it.
	Diagnostics io.Writer
}

// Server is one ACP connection to one client.
type Server struct {
	backend Backend
	opts    Options
	conn    *conn
	ctx     context.Context

	lifecycle    sync.Mutex
	capabilities json.RawMessage
	mu           sync.Mutex
	sessions     map[string]*session
	models       []api.ConfigModel
}

// Run serves ACP on in and out until in ends or ctx is done. Diagnostics must
// go to stderr by the caller: out carries only protocol messages.
func Run(ctx context.Context, in io.Reader, out io.Writer, b Backend, o Options) error {
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &Server{backend: b, opts: o, conn: newConn(out), ctx: ctx, sessions: map[string]*session{}}
	s.conn.OnHandled = (&diagnostics{out: o.Diagnostics}).record
	cleanup := sync.OnceFunc(func() { s.shutdown(); cancel() })
	defer cleanup()
	stop := context.AfterFunc(parent, func() {
		cleanup()
		if closer, ok := in.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	defer stop()
	return s.conn.Serve(in, s.methods(), cleanup)
}

// shutdown detaches every session: executing runs stop, paused questions stay.
func (s *Server) shutdown() {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
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
	var p struct {
		Version      json.RawMessage `json:"protocolVersion"`
		Capabilities json.RawMessage `json:"clientCapabilities"`
	}
	if len(params) == 0 || params[0] != '{' || json.Unmarshal(params, &p) != nil {
		return nil, errParams("initialize params must be an object")
	}
	// ACP version negotiation: when the requested version is not supported (or
	// absent), the agent answers with the latest version it supports and the
	// client decides whether to continue. A non-numeric version is malformed.
	if len(p.Version) > 0 && string(p.Version) != "null" {
		var n float64
		if json.Unmarshal(p.Version, &n) != nil {
			return nil, errParams("protocolVersion must be an integer")
		}
	}
	// ACP defaults malformed optional capabilities. No optional client requests
	// are issued, even when filesystem/terminal capabilities are present.
	var caps map[string]json.RawMessage
	if json.Unmarshal(p.Capabilities, &caps) != nil || caps == nil {
		p.Capabilities = json.RawMessage(`{}`)
	}
	s.mu.Lock()
	s.capabilities = append(json.RawMessage(nil), p.Capabilities...)
	s.mu.Unlock()

	return map[string]any{
		"protocolVersion": ProtocolVersion,
		"agentInfo":       map[string]any{"name": "meno", "title": "Meno", "version": s.opts.Version},
		"authMethods":     []any{},
		"agentCapabilities": map[string]any{
			"loadSession":         true,
			"promptCapabilities":  map[string]any{"image": true, "audio": false, "embeddedContext": true},
			"mcpCapabilities":     map[string]any{"http": true, "sse": true},
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
	if !filepath.IsAbs(p.Cwd) {
		return errParams("cwd must be absolute")
	}
	return nil
}

func clientID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (s *Server) newSession(raw json.RawMessage) (any, error) {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	var p newParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errParams("invalid params: %v", err)
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	res, err := s.backend.CreateSession(s.ctx, api.SessionCreateRequest{Cwd: p.Cwd, ClientID: clientID(), NoAsk: s.opts.NoAsk})
	if err != nil {
		return nil, err
	}
	scope, err := s.configureMCP(res.Session.ID, p.Cwd, p.MCPServers, nil)
	if err != nil {
		return nil, err
	}
	x, err := s.attach(res.Session.ID, false)
	if err != nil {
		scope.close()
		return nil, err
	}
	x.scope = scope
	out := s.sessionResult(x)
	out["sessionId"] = x.id
	return afterReply{Value: out, Then: func() { s.sendCommands(x) }}, nil
}

func (s *Server) loadSession(raw json.RawMessage) (any, error) {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	var p newParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errParams("invalid params: %v", err)
	}
	if p.SessionID == "" {
		return nil, errParams("sessionId is required")
	}
	snap, err := s.backend.Snapshot(s.ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	if p.Cwd != "" && snap.Cwd != "" && p.Cwd != snap.Cwd {
		return nil, errParams("session %s belongs to %s, not %s", p.SessionID, snap.Cwd, p.Cwd)
	}
	s.mu.Lock()
	old := s.sessions[p.SessionID]
	s.mu.Unlock()
	if old != nil {
		old.mu.Lock()
		busy := old.waiter != nil || old.running
		old.mu.Unlock()
		if busy {
			return nil, errParams("cannot load an active session")
		}
	}
	cwd := p.Cwd
	if cwd == "" {
		cwd = snap.Cwd
	}
	var previous *clientScope
	if old != nil {
		previous = old.scope
	}
	scope, err := s.configureMCP(p.SessionID, cwd, p.MCPServers, previous)
	if err != nil {
		return nil, err
	}
	// Replacing the lease already stopped the old transports. Do not release
	// the newly installed lease through the old adapter object.
	if old != nil {
		old.scope = nil
	}
	s.detachExisting(p.SessionID)
	x, err := s.attach(p.SessionID, true)
	if err != nil {
		scope.close()
		return nil, err
	}
	x.scope = scope
	return afterReply{Value: s.sessionResult(x), Then: func() { s.sendCommands(x) }}, nil
}

func (s *Server) detachExisting(id string) {
	s.mu.Lock()
	old := s.sessions[id]
	delete(s.sessions, id)
	s.mu.Unlock()
	if old != nil {
		old.detach()
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
