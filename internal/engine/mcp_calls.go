package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"meno/internal/api"
	"meno/internal/mcp"
)

func decodeData(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}

// callMCP calls one tool of a running MCP server and stores the full result
// under an id the agent can read later with "meno mcp results".
func (e *Engine) callMCP(ctx context.Context, s *runState, server, tool string, args map[string]any) (api.MCPResult, error) {
	c, ok := e.sessionMCP(s, server).Client(server)
	if !ok {
		return api.MCPResult{}, fmt.Errorf("MCP server %q is not running", server)
	}
	result, err := c.CallResult(ctx, tool, args)
	if err != nil {
		return api.MCPResult{}, err
	}
	saved, err := (mcp.ResultStore{Dir: e.paths.File("mcp-results")}).Save(s.session.ID, server, tool, result)
	if err != nil {
		return api.MCPResult{}, err
	}
	artifacts := make([]api.MCPArtifact, 0, len(saved.Artifacts))
	for _, v := range saved.Artifacts {
		artifacts = append(artifacts, api.MCPArtifact{Index: v.Index, Kind: v.Kind, Mime: v.Mime, Size: v.Size, Path: v.Path, URI: v.URI})
	}
	return api.MCPResult{Text: result.Text, StructuredContent: result.Structured, IsError: result.IsError, ResultID: saved.ID, Artifacts: artifacts, Size: saved.Size, Notes: saved.Notes}, nil
}
