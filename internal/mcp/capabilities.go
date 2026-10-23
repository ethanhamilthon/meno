package mcp

import "encoding/json"

// Capabilities are the server capabilities from the initialize result. Each
// field is non-nil when the server declared that capability.
type Capabilities struct {
	Tools       json.RawMessage `json:"tools,omitempty"`
	Resources   json.RawMessage `json:"resources,omitempty"`
	Prompts     json.RawMessage `json:"prompts,omitempty"`
	Logging     json.RawMessage `json:"logging,omitempty"`
	Completions json.RawMessage `json:"completions,omitempty"`
	// Declared is true when the server sent any capability at all.
	Declared bool `json:"-"`
}

func parseCapabilities(raw json.RawMessage) Capabilities {
	var c Capabilities
	_ = json.Unmarshal(raw, &c)
	var all map[string]json.RawMessage
	if json.Unmarshal(raw, &all) == nil && len(all) > 0 {
		c.Declared = true
	}
	return c
}

// listsTools reports whether tools/list should be called. A server that
// declares capabilities but not tools (for example only resources or prompts)
// must not be asked for tools. A server that declares nothing at all is still
// asked, because many tool-only servers omit the declaration; a "method not
// found" answer then means "no tools" (see isMethodNotFound).
func (c Capabilities) listsTools() bool {
	return c.Tools != nil || !c.Declared
}

// Capabilities returns what the server declared during initialize.
func (c *Client) Capabilities() Capabilities {
	c.toolsMu.RLock()
	defer c.toolsMu.RUnlock()
	return c.caps
}

// isMethodNotFound reports a JSON-RPC "method not found" error.
func isMethodNotFound(err error) bool {
	e := asRPC(err)
	return e != nil && e.Code == -32601
}
