package engine

import (
	"context"
	"encoding/json"
	"time"

	"meno/internal/api"
	"meno/internal/mcp"
)

// A lease bounds orphaned resources if the stdio adapter dies without cleanup.
// Credentials live only in this manager, never in session/config persistence.
const mcpLease = 45 * time.Second

type mcpScope struct {
	manager  *mcp.Manager
	owner    string
	timer    *time.Timer
	cancel   context.CancelFunc
	deadline time.Time
}

func (e *Engine) scopeResource(ctx context.Context, s *runState, method string, q api.ResourceRequest) (api.ResourceResponse, error) {
	if q.Token != "" {
		return api.ResourceResponse{}, dapi(api.CodeForbidden, "client MCP leases are client-only")
	}
	var p struct {
		Previous string            `json:"previous"`
		Owner    string            `json:"owner"`
		Servers  []json.RawMessage `json:"servers"`
	}
	if json.Unmarshal(q.Data, &p) != nil || len(p.Owner) < 16 || len(p.Owner) > 128 {
		return api.ResourceResponse{}, dapi(api.CodeBadRequest, "invalid client MCP lease")
	}
	s.scopeMu.Lock()
	defer s.scopeMu.Unlock()
	if e.isClosed() {
		return api.ResourceResponse{}, dapi(api.CodeConflict, "engine is closed")
	}
	s.mu.Lock()
	old := s.scope
	cwd := s.cwd
	s.mu.Unlock()
	if old != nil && old.owner != p.Owner && !(method == "PUT" && old.owner == p.Previous) {
		return api.ResourceResponse{}, dapi(api.CodeConflict, "session already has a client MCP owner")
	}
	switch method {
	case "PATCH":
		if old == nil {
			return api.ResourceResponse{}, dapi(api.CodeConflict, "client MCP lease expired")
		}
		old.deadline = time.Now().Add(mcpLease)
		old.timer.Reset(mcpLease)
	case "DELETE":
		e.clearScope(s, old)
	case "PUT":
		s.mu.Lock()
		running := s.running
		if !running {
			s.scopeChanging = true
		}
		s.mu.Unlock()
		defer func() { s.mu.Lock(); s.scopeChanging = false; s.mu.Unlock() }()
		if running && (old != nil || len(p.Servers) > 0) {
			return api.ResourceResponse{}, dapi(api.CodeConflict, "cannot replace MCP while session is running")
		}
		servers, err := mcp.ParseACPServers(p.Servers, cwd)
		if err != nil {
			return api.ResourceResponse{}, dapi(api.CodeBadRequest, err.Error())
		}
		globals, err := e.mcpServers(ctx)
		if err != nil {
			return api.ResourceResponse{}, dapi(api.CodeInternal, "cannot inspect global MCP configuration")
		}
		for _, v := range servers {
			for _, g := range globals {
				if g.Name == v.Name {
					return api.ResourceResponse{}, dapi(api.CodeConflict, "client MCP name conflicts with global MCP")
				}
			}
			if _, ok := e.mcp.Client(v.Name); ok {
				return api.ResourceResponse{}, dapi(api.CodeConflict, "client MCP name conflicts with global MCP")
			}
		}
		if len(servers) == 0 {
			e.clearScope(s, old)
			break
		}
		manager := mcp.NewManager()
		// Keep successful transports alive beyond setup; a separate timer cancels
		// stalled startup without tying their lifetime to this HTTP request.
		lifetime, cancel := context.WithCancel(context.Background())
		timer := time.AfterFunc(15*time.Second, cancel)
		for _, srv := range servers {
			if ctx.Err() != nil || manager.Start(lifetime, srv) != nil {
				timer.Stop()
				cancel()
				manager.StopAll()
				return api.ResourceResponse{}, dapi(api.CodeBadRequest, "client MCP setup failed; check server transport and credentials")
			}
		}
		if !timer.Stop() || lifetime.Err() != nil {
			cancel()
			manager.StopAll()
			return api.ResourceResponse{}, dapi(api.CodeBadRequest, "client MCP setup timed out")
		}
		// StopAll cancels each transport; cancel releases the parent context too.
		scope := &mcpScope{manager: manager, owner: p.Owner, cancel: cancel, deadline: time.Now().Add(mcpLease)}
		scope.timer = time.AfterFunc(mcpLease, func() {
			s.scopeMu.Lock()
			defer s.scopeMu.Unlock()
			if remaining := time.Until(scope.deadline); remaining > 0 {
				scope.timer.Reset(remaining)
				return
			}
			e.clearScope(s, scope)
		})
		e.clearScope(s, old)
		s.mu.Lock()
		s.scope = scope
		s.mu.Unlock()
	default:
		return api.ResourceResponse{}, dapi(api.CodeBadRequest, "unsupported client MCP lease operation")
	}
	return api.ResourceResponse{Data: json.RawMessage(`{}`)}, nil
}

// clearScope is called with scopeMu held. A stale timer cannot clear a new lease.
func (e *Engine) clearScope(s *runState, scope *mcpScope) {
	if scope == nil {
		return
	}
	s.mu.Lock()
	if s.scope == scope {
		s.scope = nil
	}
	s.mu.Unlock()
	scope.timer.Stop()
	scope.manager.StopAll()
	scope.cancel()
}
func (e *Engine) sessionMCP(s *runState, name string) *mcp.Manager {
	s.mu.Lock()
	scope := s.scope
	s.mu.Unlock()
	if scope != nil {
		if _, ok := scope.manager.Client(name); ok {
			return scope.manager
		}
	}
	return e.mcp
}
func (e *Engine) scopedManager(s *runState) *mcp.Manager {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scope != nil {
		return s.scope.manager
	}
	return nil
}
