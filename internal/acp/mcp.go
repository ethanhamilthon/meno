package acp

import (
	"context"
	"encoding/json"
	"errors"
	"meno/internal/client"
	"path/filepath"
	"sync"
	"time"

	"meno/internal/api"
	"meno/internal/mcp"
)

type resourceBackend interface {
	Resource(context.Context, string, string, string, api.ResourceRequest) (api.ResourceResponse, error)
}

// clientScope owns one daemon lease. Config and credentials are not retained by
// the adapter after setup. EOF releases it; heartbeat expiry handles a crash.
type clientScope struct {
	backend   resourceBackend
	id, owner string
	stop      chan struct{}
	once      sync.Once
}

func (s *Server) configureMCP(id, cwd string, raw []json.RawMessage, previous *clientScope) (*clientScope, error) {
	if !filepath.IsAbs(cwd) {
		return nil, errParams("cwd must be absolute")
	}
	if _, err := mcp.ParseACPServers(raw, cwd); err != nil {
		return nil, errParams("%s", err)
	}
	b, ok := s.backend.(resourceBackend)
	if !ok {
		if len(raw) == 0 {
			return nil, nil
		}
		return nil, &rpcError{Code: codeInternal, Message: "daemon does not support client MCP leases"}
	}
	scope := &clientScope{backend: b, id: id, owner: clientID(), stop: make(chan struct{})}
	data, _ := json.Marshal(map[string]any{"owner": scope.owner, "previous": previousOwner(previous), "servers": raw})
	if _, err := b.Resource(s.ctx, "PUT", "/v1/sessions/"+id+"/client-mcp", "", api.ResourceRequest{Data: data}); err != nil {
		var remote *client.RemoteError
		if errors.As(err, &remote) {
			switch remote.Code {
			case api.CodeConflict:
				return nil, errParams("client MCP conflict: another attachment owns it, the session is running, or a name conflicts with global MCP")
			case api.CodeBadRequest:
				return nil, errParams("client MCP setup failed: check configuration, transport, credentials and startup timeout")
			case api.CodeNotFound:
				return nil, errParams("client MCP unavailable: session not found or daemon needs a manual upgrade/restart")
			}
		}
		return nil, &rpcError{Code: codeInternal, Message: "client MCP setup failed: daemon unavailable"}
	}
	if previous != nil {
		previous.retire()
	}
	if len(raw) == 0 {
		return nil, nil
	}
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-scope.stop:
				return
			case <-s.ctx.Done():
				scope.close()
				return
			case <-t.C:
				ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
				err := scope.request(ctx, "PATCH", nil)
				cancel()
				if err != nil {
					scope.close()
					return
				}
			}
		}
	}()
	return scope, nil
}
func (c *clientScope) request(ctx context.Context, method string, servers []json.RawMessage) error {
	data, _ := json.Marshal(map[string]any{"owner": c.owner, "servers": servers})
	_, err := c.backend.Resource(ctx, method, "/v1/sessions/"+c.id+"/client-mcp", "", api.ResourceRequest{Data: data})
	return err
}
func (c *clientScope) close() {
	if c == nil {
		return
	}
	c.once.Do(func() {
		close(c.stop)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = c.request(ctx, "DELETE", nil)
	})
}

func previousOwner(c *clientScope) string {
	if c != nil {
		return c.owner
	}
	return ""
}

// retire stops heartbeat without deleting the replacement lease. A late old
// DELETE is also harmless because each successful attachment has a fresh owner.
func (c *clientScope) retire() {
	if c != nil {
		c.once.Do(func() { close(c.stop) })
	}
}
