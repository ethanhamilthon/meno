package llm

import "encoding/json"

// ExportToolDefinition returns the same function definition used by both wire
// protocols, without exposing the request's provider or authentication data.
func (r Request) ExportToolDefinition() json.RawMessage {
	if !r.Tools {
		return nil
	}
	b, _ := json.Marshal(r.toolSchema())
	return b
}
