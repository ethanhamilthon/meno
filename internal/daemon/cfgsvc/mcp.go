package cfgsvc

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"

	"meno/internal/config"
	"meno/internal/mcp"
	"meno/internal/proto"
)

func (s *Service) servers() ([]mcp.Server, error) {
	all, _, err := s.serverDefinitions()
	return all, err
}
func (s *Service) serverDefinitions() ([]mcp.Server, []string, error) {
	own, err := s.store.LoadServers()
	if err != nil {
		return nil, nil, err
	}
	files, warnings := config.LoadMCPFile(s.paths.MCPFile())
	all, more := config.MergeServers(own, files)
	return all, append(warnings, more...), nil
}
func (s *Service) mcpResource(ctx context.Context, method, path string, req proto.ResourceRequest) (any, error) {
	if method != "GET" {
		s.mcpMu.Lock()
		defer s.mcpMu.Unlock()
	}
	parts := splitResource(strings.TrimPrefix(path, "/mcp/servers"))
	if len(parts) == 0 {
		if method == "GET" {
			return s.mcpResponse()
		}
		if method == "POST" {
			request, e := decode[proto.McpServerRequest](req.Data)
			if e != nil {
				return nil, bad(e)
			}
			r := request.Server
			r.Source = ""
			x := mcpValue(r)
			if e = mcp.ValidateServer(x); e != nil {
				return nil, bad(e)
			}
			list, e := s.store.LoadServers()
			if e != nil {
				return nil, e
			}
			if serverIndex(list, x.Name) >= 0 {
				return nil, conflict("MCP server already exists: " + x.Name)
			}
			list = append(list, x)
			if e = s.store.SaveServers(list); e != nil {
				return nil, e
			}
			s.changed("mcp_changed")
			if x.Enabled {
				if e = s.startMCP(ctx, x); e != nil {
					return nil, e
				}
			}
			return proto.McpServerResponse{Server: s.mcpWire(x)}, nil
		}
		return nil, notFound("unsupported MCP route")
	}
	name, e := url.PathUnescape(parts[0])
	if e != nil {
		return nil, bad(e)
	}
	list, e := s.servers()
	if e != nil {
		return nil, e
	}
	i := serverIndex(list, name)
	if i < 0 {
		return nil, notFound("no MCP server named " + name)
	}
	x := list[i]
	if req.ExpectedMCP != nil {
		current := s.mcpWire(x)
		current.Running, current.Starting, current.Error = false, false, ""
		if !reflect.DeepEqual(current, *req.ExpectedMCP) {
			return nil, conflict("MCP definition changed after approval")
		}
	}
	if x.FromFile() && method != "GET" && !(len(parts) == 2 && parts[1] == "restart" && method == "POST") {
		return nil, apiErr(proto.CodeForbidden, "MCP server is defined in mcp.json and is read only")
	}
	if len(parts) == 1 {
		switch method {
		case "GET":
			return proto.McpServerResponse{Server: s.mcpWire(x)}, nil
		case "PUT":
			request, e := decode[proto.McpServerRequest](req.Data)
			if e != nil {
				return nil, bad(e)
			}
			r := request.Server
			r.Source = ""
			next := mcpValue(r)
			if next.Name != name {
				return nil, badString("MCP server name cannot change")
			}
			if e = mcp.ValidateServer(next); e != nil {
				return nil, bad(e)
			}
			own, e := s.store.LoadServers()
			if e != nil {
				return nil, e
			}
			j := serverIndex(own, name)
			if j < 0 {
				return nil, notFound("no MCP server named " + name)
			}
			if reflect.DeepEqual(own[j], next) {
				return proto.McpServerResponse{Server: s.mcpWire(next)}, nil
			}
			own[j] = next
			if e = s.store.SaveServers(own); e != nil {
				return nil, e
			}
			s.mcp.Stop(name)
			if next.Enabled {
				if e = s.startMCP(ctx, next); e != nil {
					return nil, e
				}
			}
			s.changed("mcp_changed")
			return proto.McpServerResponse{Server: s.mcpWire(next)}, nil
		case "DELETE":
			own, e := s.store.LoadServers()
			if e != nil {
				return nil, e
			}
			j := serverIndex(own, name)
			if j < 0 {
				return nil, notFound("no MCP server named " + name)
			}
			own = append(own[:j], own[j+1:]...)
			if e = s.store.SaveServers(own); e != nil {
				return nil, e
			}
			s.mcp.Stop(name)
			s.changed("mcp_changed")
			return proto.AckResponse{OK: true}, nil
		}
	}
	if len(parts) == 2 && parts[1] == "restart" && method == "POST" {
		if !x.Enabled {
			return nil, badString("MCP server is disabled")
		}
		if e = s.startMCP(ctx, x); e != nil {
			return nil, e
		}
		s.changed("mcp_changed")
		return proto.AckResponse{OK: true}, nil
	}
	return nil, notFound("unsupported MCP route")
}
func (s *Service) startMCP(_ context.Context, x mcp.Server) error {
	s.mu.Lock()
	runCtx := s.ctx
	s.mu.Unlock()
	if runCtx == nil {
		runCtx = context.Background()
	}
	return s.mcp.Start(runCtx, x)
}
func serverIndex(xs []mcp.Server, name string) int {
	for i := range xs {
		if xs[i].Name == name {
			return i
		}
	}
	return -1
}
func (s *Service) mcpWire(x mcp.Server) proto.ConfigMCPServer {
	b, _ := json.Marshal(x)
	var out proto.ConfigMCPServer
	_ = json.Unmarshal(b, &out)
	out.Source = x.Source
	status := s.mcp.Snapshot(x.Name)
	out.Running, out.Starting, out.Error = status.Running, status.Starting, status.Error
	return out
}
func mcpValue(x proto.ConfigMCPServer) mcp.Server {
	b, _ := json.Marshal(x)
	var out mcp.Server
	_ = json.Unmarshal(b, &out)
	out.Source = x.Source
	return out
}
