package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// httpError is a non-2xx answer of a streamable-http server.
type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("http %d: %s", e.Status, strings.TrimSpace(e.Body))
}

// rpc returns the JSON-RPC error carried in the body, or nil.
func (e *httpError) rpc() *rpcError {
	var m struct {
		Error *rpcError `json:"error"`
	}
	if json.Unmarshal([]byte(e.Body), &m) != nil {
		return nil
	}
	return m.Error
}
