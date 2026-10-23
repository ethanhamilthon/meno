package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"meno/internal/version"
)

// modernVersion is the 2026-07-28 revision: no initialize handshake and no
// protocol session; every request carries its version and client identity in
// _meta. Supported here: server/discover, tools/list, tools/call, resultType
// "complete", the HTTP request headers and x-mcp-header. Not supported:
// subscriptions/listen, multi round-trip input (input_required), extensions.
const modernVersion = "2026-07-28"

// Values of Server.Protocol.
const (
	ProtocolLegacy = "legacy" // initialize handshake only
	ProtocolAuto   = "auto"   // try modern first, fall back to legacy; also the default ("")
	ProtocolModern = "modern" // modern only
)

// probeTimeout bounds the modern probe; a legacy server may stay silent.
var probeTimeout = 10 * time.Second

const (
	metaVersion = "io.modelcontextprotocol/protocolVersion"
	metaClient  = "io.modelcontextprotocol/clientInfo"
	metaCaps    = "io.modelcontextprotocol/clientCapabilities"
)

// wantsModern reports whether to try the modern era first: unless the server
// is pinned to "legacy". The sse transport has no modern form.
func (s Server) wantsModern() bool {
	return s.Protocol != ProtocolLegacy && s.Transport != TransportSSE
}

// withMeta adds the per-request metadata of the modern era to params.
func (c *Client) withMeta(params any) any {
	if !c.modern {
		return params
	}
	out := map[string]any{}
	if m, ok := params.(map[string]any); ok {
		for k, v := range m {
			out[k] = v
		}
	}
	out["_meta"] = map[string]any{
		metaVersion: modernVersion,
		metaClient:  map[string]any{"name": "meno", "version": version.Number()},
		metaCaps:    map[string]any{},
	}
	return out
}

// discover runs server/discover, the modern replacement of initialize.
func (c *Client) discover(ctx context.Context) error {
	raw, err := c.request(ctx, "server/discover", map[string]any{}, probeTimeout)
	if err != nil {
		return err
	}
	var d struct {
		Supported []string        `json:"supportedVersions"`
		Caps      json.RawMessage `json:"capabilities"`
	}
	if err = json.Unmarshal(raw, &d); err != nil {
		return err
	}
	if !slices.Contains(d.Supported, modernVersion) {
		return &notModernError{fmt.Sprintf("mcp: server supports %v, not %s", d.Supported, modernVersion)}
	}
	c.toolsMu.Lock()
	c.caps = parseCapabilities(d.Caps)
	c.toolsMu.Unlock()
	return nil
}

// isModernError reports a JSON-RPC error that only a modern server sends
// (HeaderMismatch, MissingRequiredClientCapability, UnsupportedProtocolVersion).
// It identifies a modern server that must not be retried as legacy.
func isModernError(err error) bool {
	e := asRPC(err)
	return e != nil && e.Code >= -32022 && e.Code <= -32020
}

// resultType returns the resultType of a result; servers of earlier versions
// omit it, which means "complete".
func resultType(raw json.RawMessage) string {
	var r struct {
		Type string `json:"resultType"`
	}
	if json.Unmarshal(raw, &r) != nil || r.Type == "" {
		return "complete"
	}
	return r.Type
}

var errInputRequired = errors.New("mcp: the server asks for more input (multi round-trip request), which meno does not support yet")

// startModern discovers the server and lists its tools; there is no handshake.
func (c *Client) startModern(ctx context.Context) error {
	if err := c.discover(ctx); err != nil {
		return err
	}
	return c.refresh(ctx)
}

// notModernError is a successful answer that is not a modern discover result:
// a legacy server that ignores unknown fields and methods.
type notModernError struct{ msg string }

func (e *notModernError) Error() string { return e.msg }

// fallsBackToLegacy decides after a failed modern start. Per the specification
// a recognized modern error identifies a modern server (no fallback); on HTTP
// only a 400/404/405 or a JSON-RPC error answer means "legacy" (a 401, a
// network failure or a 5xx is a real error); on stdio any other error or
// silence does.
func fallsBackToLegacy(s Server, err error) bool {
	if s.Protocol == ProtocolModern || isModernError(err) {
		return false
	}
	var nm *notModernError
	if errors.As(err, &nm) {
		return true
	}
	if s.Transport != TransportHTTP {
		return true
	}
	e := asRPC(err)
	if e == nil || e.local {
		return false
	}
	return e.status == 0 || e.status == 400 || e.status == 404 || e.status == 405
}

// asRPC views an error of a request as a JSON-RPC error: a JSON-RPC error
// answer, or an HTTP status answer (with the error of its body when it has
// one). Transport failures give nil or a local error.
func asRPC(err error) *rpcError {
	var h *httpError
	if errors.As(err, &h) {
		if re := h.rpc(); re != nil {
			re.status = h.Status
			return re
		}
		return &rpcError{Code: -32000, Message: h.Error(), status: h.Status}
	}
	var e *rpcError
	if errors.As(err, &e) {
		return e
	}
	return nil
}

// headerIndex maps a tool to its x-mcp-header annotations.
type headerIndex map[string][]headerSpec

// usableTools drops tools whose x-mcp-header annotations are invalid, which the
// modern HTTP binding requires; other tools and transports keep every tool.
func (c *Client) usableTools(tools []Tool) []Tool {
	if !c.modern || c.sendRemote == nil {
		return tools
	}
	out := tools[:0:0]
	for _, t := range tools {
		specs, err := headerSpecs(t.InputSchema)
		if err != nil {
			c.rejected = append(c.rejected, t.Name+": "+err.Error())
			continue
		}
		if len(specs) > 0 {
			c.pending[t.Name] = specs
		}
		out = append(out, t)
	}
	return out
}

// RejectedTools lists tools left out because their definition is invalid.
func (c *Client) RejectedTools() []string {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	return append([]string(nil), c.rejected...)
}

func (c *Client) specsFor(tool string) []headerSpec {
	c.toolsMu.RLock()
	defer c.toolsMu.RUnlock()
	return c.headerSpecs[tool]
}

// startHTTPModern connects with the modern protocol: no initialize and no
// session; the first request is server/discover.
func startHTTPModern(ctx context.Context, s Server) (*Client, error) {
	r, c := newRemote(ctx, s)
	c.modern, r.modern = true, true
	r.version, r.reinit = modernVersion, nil
	r.specs = c.specsFor
	if err := c.startModern(ctx); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
