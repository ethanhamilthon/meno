package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// eraServer is a fake streamable-http server. modern serves the 2026-07-28 era,
// legacy the initialize era; both can be on at once (dual-era).
type eraServer struct {
	mu      sync.Mutex
	modern  bool
	legacy  bool
	schema  map[string]any
	seen    []http.Header
	methods []string
	meta    []map[string]any
}

func (e *eraServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     *int64 `json:"id"`
		Method string `json:"method"`
		Params struct {
			Meta map[string]any `json:"_meta"`
			Name string         `json:"name"`
		} `json:"params"`
	}
	b, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(b, &req)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.methods = append(e.methods, req.Method)
	e.seen = append(e.seen, r.Header.Clone())
	e.meta = append(e.meta, req.Params.Meta)
	write := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	isModern := req.Params.Meta[metaVersion] != nil
	switch {
	case isModern && e.modern:
		if r.Header.Get("Mcp-Method") != req.Method {
			write(400, map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32020, "message": "Header mismatch"}})
			return
		}
		var result any
		switch req.Method {
		case "server/discover":
			result = map[string]any{"resultType": "complete", "supportedVersions": []string{modernVersion}, "capabilities": map[string]any{"tools": map[string]any{}}}
		case "tools/list":
			schema := e.schema
			if schema == nil {
				schema = map[string]any{"type": "object"}
			}
			result = map[string]any{"resultType": "complete", "tools": []map[string]any{{"name": "echo", "inputSchema": schema}, {"name": "bad", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"a": map[string]any{"type": "number", "x-mcp-header": "A"}}}}}}
		case "tools/call":
			if req.Params.Name == "ask" {
				result = map[string]any{"resultType": "input_required", "inputRequests": map[string]any{}}
			} else {
				result = map[string]any{"resultType": "complete", "content": []map[string]any{{"type": "text", "text": "modern"}}}
			}
		}
		write(200, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	case !isModern && e.legacy:
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		write(200, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": reply(req.Method, nil)})
	case isModern && e.legacy:
		http.Error(w, "bad request", http.StatusBadRequest) // legacy server, not a modern error
	default: // initialize sent to a modern-only server
		write(400, map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32600, "message": "use " + modernVersion}})
	}
}

func startEra(t *testing.T, e *eraServer, protocol string) (*Client, error) {
	t.Helper()
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	return Start(context.Background(), Server{Name: "r", Transport: TransportHTTP, URL: srv.URL, Enabled: true, Protocol: protocol})
}

func TestModernHTTPOnlyServer(t *testing.T) {
	e := &eraServer{modern: true, schema: map[string]any{"type": "object", "properties": map[string]any{"region": map[string]any{"type": "string", "x-mcp-header": "Region"}}}}
	c, err := startEra(t, e, ProtocolModern)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if tools := c.ToolSnapshot(); len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("invalid x-mcp-header tool must be excluded: %+v", tools)
	}
	if len(c.RejectedTools()) != 1 {
		t.Fatalf("rejected %v", c.RejectedTools())
	}
	res, err := c.CallResult(context.Background(), "echo", map[string]any{"region": "us-west1"})
	if err != nil || res.Text != "modern" {
		t.Fatalf("%+v %v", res, err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	last := e.seen[len(e.seen)-1]
	if last.Get("Mcp-Method") != "tools/call" || last.Get("Mcp-Name") != "echo" || last.Get("Mcp-Param-Region") != "us-west1" || last.Get("MCP-Protocol-Version") != modernVersion || last.Get("Mcp-Session-Id") != "" {
		t.Fatalf("headers %v", last)
	}
	for _, m := range e.methods {
		if m == "initialize" {
			t.Fatal("modern must not initialize")
		}
	}
}

func TestModernInputRequiredIsNotTakenAsResult(t *testing.T) {
	c, err := startEra(t, &eraServer{modern: true}, ProtocolModern)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err = c.CallResult(context.Background(), "ask", nil); err == nil || !strings.Contains(err.Error(), "multi round-trip") {
		t.Fatalf("err %v", err)
	}
}

func TestAutoFallsBackToLegacyHTTP(t *testing.T) {
	e := &eraServer{legacy: true}
	c, err := startEra(t, e, ProtocolAuto)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.modern || len(c.ToolSnapshot()) != 1 {
		t.Fatalf("modern=%v tools=%v", c.modern, c.ToolSnapshot())
	}
}

func TestEmptyProtocolMeansAuto(t *testing.T) {
	c, err := startEra(t, &eraServer{modern: true, legacy: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !c.modern {
		t.Fatal("default must be auto")
	}
	c2, err := startEra(t, &eraServer{legacy: true}, "")
	if err != nil || c2.modern {
		t.Fatalf("legacy-only server under default: %v", err)
	}
	c2.Close()
}

func TestAutoPrefersModernOnDualEraHTTP(t *testing.T) {
	c, err := startEra(t, &eraServer{modern: true, legacy: true}, ProtocolAuto)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !c.modern {
		t.Fatal("dual-era server must be used in the modern era")
	}
}

func TestModernOnlyServerRejectsLegacyClient(t *testing.T) {
	if _, err := startEra(t, &eraServer{modern: true}, ProtocolLegacy); err == nil {
		t.Fatal("legacy client must not connect to a modern-only server")
	}
}

func TestAutoDoesNotFallBackOnAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", 401) }))
	defer srv.Close()
	_, err := Start(context.Background(), Server{Name: "r", Transport: TransportHTTP, URL: srv.URL, Enabled: true, Protocol: ProtocolAuto})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("%v", err)
	}
}

func TestHeaderSpecsAndEncoding(t *testing.T) {
	good := `{"type":"object","properties":{"a":{"type":"string","x-mcp-header":"Region"},"o":{"type":"object","properties":{"n":{"type":"integer","x-mcp-header":"N"},"b":{"type":"boolean","x-mcp-header":"B"}}}}}`
	specs, err := headerSpecs(json.RawMessage(good))
	if err != nil || len(specs) != 3 {
		t.Fatalf("%v %v", specs, err)
	}
	h := paramHeaders(specs, map[string]any{"a": "Hello, 世界", "o": map[string]any{"n": float64(42), "b": true}})
	if h["Mcp-Param-Region"] != "=?base64?SGVsbG8sIOS4lueVjA==?=" || h["Mcp-Param-N"] != "42" || h["Mcp-Param-B"] != "true" {
		t.Fatalf("%v", h)
	}
	if got := paramHeaders(specs, map[string]any{"a": nil}); len(got) != 0 {
		t.Fatalf("null must omit: %v", got)
	}
	if encodeHeaderValue(" padded ") != "=?base64?IHBhZGRlZCA=?=" || encodeHeaderValue("=?base64?literal?=") != "=?base64?PT9iYXNlNjQ/bGl0ZXJhbD89?=" {
		t.Fatal("sentinel encoding")
	}
	for _, bad := range []string{
		`{"properties":{"a":{"type":"number","x-mcp-header":"A"}}}`,
		`{"properties":{"a":{"type":"string","x-mcp-header":""}}}`,
		`{"properties":{"a":{"type":"string","x-mcp-header":"a b"}}}`,
		`{"properties":{"a":{"type":"string","x-mcp-header":"X"},"b":{"type":"string","x-mcp-header":"x"}}}`,
		`{"properties":{"a":{"type":"array","items":{"type":"string","x-mcp-header":"X"}}}}`,
		`{"anyOf":[{"properties":{"a":{"type":"string","x-mcp-header":"X"}}}]}`,
	} {
		if _, err := headerSpecs(json.RawMessage(bad)); err == nil {
			t.Fatalf("must be invalid: %s", bad)
		}
	}
}
