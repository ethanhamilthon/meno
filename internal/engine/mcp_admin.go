package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"meno/internal/api"
	"meno/internal/daemon"
	"meno/internal/form"
	"net/url"
	"reflect"
	"time"
)

// approvalTimeout bounds the question that guards a configuration change. A
// question nobody answers must end, not stall the agent.
const approvalTimeout = 2 * time.Minute

// mcpStatus reports the state of one server after a change.
func mcpStatus(e *Engine, name string) string {
	if e.mcp.Running(name) {
		return "running"
	}
	if e.mcp.Starting(name) {
		return "starting"
	}
	return "stopped"
}

// adminMCP changes one MCP server. A restart only reconnects a server that is
// already configured, so it runs without a question; adding or removing a
// server asks the user first.
func (e *Engine) adminMCP(ctx context.Context, s *runState, op, name string) (any, error) {
	if e.resources == nil {
		return nil, dapi(api.CodeConflict, "MCP configuration service is unavailable")
	}
	path := "/v1/mcp/servers/" + url.PathEscape(name)
	read := func() (api.ConfigMCPServer, error) {
		r, err := e.resources.Resource(ctx, "GET", path, "", api.ResourceRequest{})
		var out api.McpServerResponse
		if err == nil {
			err = json.Unmarshal(r.Data, &out)
		}
		return out.Server, err
	}
	before, err := read()
	if err != nil {
		var ae *daemon.APIError
		if errors.As(err, &ae) && ae.Code == api.CodeNotFound {
			return nil, dapi(api.CodeNotFound, fmt.Sprintf("no MCP server %q in %s; to create one add an entry to that file (see meno mcp add --help), then run: meno mcp restart %s", name, e.paths.File("mcp.json"), name))
		}
		return nil, fmt.Errorf("MCP server %q is not available: %w", name, err)
	}
	if op != "restart" {
		if s.session.NoAsk {
			return nil, dapi(api.CodeForbidden, "changing MCP servers needs the user's approval and asking is turned off for this session (--no-ask). Tell the user in the chat what to change; they can use /mcp.")
		}
		// Adding or removing a server changes the configuration and needs the
		// user. The question is bounded: an unanswered question must not hold
		// the agent forever.
		answers, err := (asker{e, s}).Ask(ctx, form.Form{
			Timeout:   approvalTimeout,
			Questions: []form.Question{{ID: "approve", Text: fmt.Sprintf("Allow MCP %s for %s?", op, name), Type: form.TypeSingle, Options: []form.Option{{ID: "yes", Label: "Allow"}, {ID: "no", Label: "Deny"}}}},
		})
		if err != nil {
			return nil, err
		}
		if len(answers) != 1 || len(answers[0].Selected) != 1 || answers[0].Selected[0] != "yes" {
			return nil, dapi(api.CodeForbidden, "MCP operation denied")
		}
	}
	after, err := read()
	if err != nil {
		return nil, err
	}
	before.Running, before.Starting, before.Error = false, false, ""
	after.Running, after.Starting, after.Error = false, false, ""
	if !reflect.DeepEqual(before, after) {
		return nil, dapi(api.CodeConflict, "MCP configuration changed; request approval again")
	}
	method := "POST"
	var data json.RawMessage
	switch op {
	case "restart":
		path += "/restart"
	case "remove":
		method = "DELETE"
	case "add":
		method = "PUT"
		after.Enabled = true
		data, _ = json.Marshal(api.McpServerRequest{Server: after})
	}
	if _, err = e.resources.Resource(ctx, method, path, "", api.ResourceRequest{Data: data, ExpectedMCP: &before}); err != nil {
		return nil, err
	}
	return map[string]any{"name": name, "status": mcpStatus(e, name)}, nil
}
