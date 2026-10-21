package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"meno/internal/form"
	"meno/internal/proto"
	"net/url"
	"reflect"
)

func (e *Engine) adminMCP(ctx context.Context, s *runState, op, name string) (any, error) {
	if e.resources == nil {
		return nil, api(proto.CodeConflict, "MCP configuration service is unavailable")
	}
	path := "/v1/mcp/servers/" + url.PathEscape(name)
	read := func() (proto.ConfigMCPServer, error) {
		r, err := e.resources.Resource(ctx, "GET", path, "", proto.ResourceRequest{})
		var out proto.McpServerResponse
		if err == nil {
			err = json.Unmarshal(r.Data, &out)
		}
		return out.Server, err
	}
	before, err := read()
	if err != nil {
		return nil, fmt.Errorf("configured MCP server %q not available; use /mcp to create it: %w", name, err)
	}
	if before.Source != "" && op != "restart" {
		return nil, api(proto.CodeForbidden, "file-defined MCP configuration is read only")
	}
	answers, err := (asker{e, s}).Ask(ctx, form.Form{Questions: []form.Question{{ID: "approve", Text: fmt.Sprintf("Allow MCP %s for %s?", op, name), Type: form.TypeSingle, Options: []form.Option{{ID: "yes", Label: "Allow"}, {ID: "no", Label: "Deny"}}}}})
	if err != nil {
		return nil, err
	}
	if len(answers) != 1 || len(answers[0].Selected) != 1 || answers[0].Selected[0] != "yes" {
		return nil, api(proto.CodeForbidden, "MCP operation denied")
	}
	after, err := read()
	if err != nil {
		return nil, err
	}
	before.Running, before.Starting, before.Error = false, false, ""
	after.Running, after.Starting, after.Error = false, false, ""
	if !reflect.DeepEqual(before, after) {
		return nil, api(proto.CodeConflict, "MCP configuration changed; request approval again")
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
		data, _ = json.Marshal(proto.McpServerRequest{Server: after})
	}
	if _, err = e.resources.Resource(ctx, method, path, "", proto.ResourceRequest{Data: data, ExpectedMCP: &before}); err != nil {
		return nil, err
	}
	status := "stopped"
	if e.mcp.Running(name) {
		status = "running"
	}
	return map[string]any{"name": name, "status": status}, nil
}
