package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"meno/internal/mcp"
)

func decodeData(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}

// mcpResult is what a tool call gives back to the agent's shell command.
type mcpResult struct {
	Text     string `json:"text"`
	ResultID string `json:"resultId"`
	IsError  bool   `json:"isError,omitempty"`
}

// callMCP calls one tool of a running MCP server and stores the full result
// under an id the agent can read later with "meno mcp results".
func (e *Engine) callMCP(ctx context.Context, s *runState, server, tool string, args map[string]any) (mcpResult, error) {
	c, ok := e.mcp.Client(server)
	if !ok {
		return mcpResult{}, fmt.Errorf("MCP server %q is not running", server)
	}
	text, err := c.Call(ctx, tool, args)
	if err != nil {
		return mcpResult{}, err
	}
	saved, err := (mcp.ResultStore{Dir: e.paths.File("mcp-results")}).Save(s.session.ID, server, tool, mcp.Result{Text: text})
	if err != nil {
		return mcpResult{}, err
	}
	return mcpResult{Text: text, ResultID: saved.ID}, nil
}
