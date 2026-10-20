package engine

import (
	"context"
	"encoding/json"
	"errors"
	"meno/internal/daemon"
	"meno/internal/proto"
	"strings"
)

func (e *Engine) Resource(ctx context.Context, method, path, query string, q proto.ResourceRequest) (proto.ResourceResponse, error) {
	if strings.HasPrefix(path, "/v1/sessions/") {
		return e.sessionResource(ctx, method, path, query, q)
	}
	if e.resources != nil {
		return e.resources.Resource(ctx, method, path, query, q)
	}
	return proto.ResourceResponse{}, api(proto.CodeNotFound, "resource is not implemented")
}
func (e *Engine) sessionResource(ctx context.Context, method, path, query string, q proto.ResourceRequest) (proto.ResourceResponse, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/v1/sessions/"), "/")
	if len(parts) < 2 {
		return proto.ResourceResponse{}, api(proto.CodeNotFound, "resource not found")
	}
	id := parts[0]
	s, err := e.load(ctx, id)
	if err != nil {
		return proto.ResourceResponse{}, notFound(err)
	}
	group := parts[1]
	if !e.authorized(s, q.Token, group) {
		return proto.ResourceResponse{}, api(proto.CodeForbidden, "resource group is forbidden by the session mode")
	}
	data := map[string]any{}
	_ = decodeData(q.Data, &data)
	var out any
	switch group {
	case "tasks":
		out, err = e.taskResource(ctx, s, parts[2:], method, data, query)
	case "agents":
		out, err = e.agentResource(ctx, s, parts[2:], method, data, query)
	case "mcp":
		out, err = e.mcpResource(ctx, s, parts[2:], method, data, query)
	case "web":
		out, err = e.webResource(ctx, s, parts[2:], method, data, query)
	case "view":
		out, err = e.viewResource(s, data)
	case "goal":
		out, err = e.goalResource(s, parts[2:], data)
	case "models":
		out = e.modelResource(query)
	case "ask":
		out, err = e.askResource(ctx, s, q.Call, data)
	default:
		return proto.ResourceResponse{}, api(proto.CodeNotFound, "resource not found")
	}
	if err != nil {
		return proto.ResourceResponse{}, resourceError(err)
	}
	b, er := json.Marshal(out)
	if er != nil {
		return proto.ResourceResponse{}, er
	}
	return proto.ResourceResponse{Data: b}, nil
}
func (e *Engine) authorized(s *runState, token, group string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return token != "" && token == s.token.value && s.token.groups[group]
}
func resourceError(err error) error {
	var apiErr *daemon.APIError
	if errors.As(err, &apiErr) {
		return err
	}
	if strings.Contains(err.Error(), "not found") {
		return api(proto.CodeNotFound, err.Error())
	}
	return err
}
