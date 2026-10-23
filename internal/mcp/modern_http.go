package mcp

import (
	"encoding/json"
	"net/http"
)

// modernHeaders adds the request headers of the modern HTTP binding:
// Mcp-Method on every request, Mcp-Name on tools/call, and the Mcp-Param-*
// headers designated by x-mcp-header.
func (r *remote) modernHeaders(req *http.Request, body []byte) {
	var m struct {
		Method string `json:"method"`
		Params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &m) != nil || m.Method == "" {
		return
	}
	req.Header.Set("Mcp-Method", m.Method)
	if m.Method != "tools/call" {
		return
	}
	req.Header.Set("Mcp-Name", encodeHeaderValue(m.Params.Name))
	if r.specs != nil {
		for k, v := range paramHeaders(r.specs(m.Params.Name), m.Params.Arguments) {
			req.Header.Set(k, v)
		}
	}
}
