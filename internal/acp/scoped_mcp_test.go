package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"meno/internal/api"
)

// sandboxMCP is a deterministic legacy streamable HTTP server, never a user server.
func sandboxMCP(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(200)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var q struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			w.WriteHeader(400)
			return
		}
		if len(q.ID) == 0 {
			w.WriteHeader(202)
			return
		}
		var res any
		switch q.Method {
		case "initialize":
			res = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "sandbox", "version": "1"}}
		case "tools/list":
			res = map[string]any{"tools": []any{map[string]any{"name": "echo", "description": "sandbox echo", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			calls.Add(1)
			res = map[string]any{"content": []any{map[string]string{"type": "text", "text": "sandbox-result"}}}
		default:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32601,"message":"not found"}}`, q.ID)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": res})
	}))
	t.Cleanup(h.Close)
	return h.URL, calls
}

func TestMCPSessionIsolationLoadAndCleanup(t *testing.T) {
	url, calls := sandboxMCP(t)
	c, _ := daemonRig(t, &gated{})
	p := startPeer(t, c)
	p.call("initialize", map[string]any{"protocolVersion": 1})
	cwd := t.TempDir()
	servers := []any{map[string]any{"type": "http", "name": "sandbox-tools", "url": url, "headers": []any{map[string]string{"name": "Authorization", "value": "sandbox-secret"}}}}
	id := result(t, p.call("session/new", map[string]any{"cwd": cwd, "mcpServers": servers}))["sessionId"].(string)
	call := func(id string) error {
		_, err := c.Resource(context.Background(), "POST", "/v1/sessions/"+id+"/mcp/call", "", api.ResourceRequest{Data: json.RawMessage(`{"server":"sandbox-tools","tool":"echo","args":{}}`)})
		return err
	}
	if err := call(id); err != nil {
		t.Fatal(err)
	}
	other := newSessionOn(t, p)
	if err := call(other); err == nil {
		t.Fatal("client MCP leaked to another session")
	}
	// Persist history, then load with fresh ephemeral configuration.
	promptText(t, p, id, "hello")
	if m := p.call("session/load", map[string]any{"sessionId": id, "cwd": cwd, "mcpServers": servers}); m.Error != nil {
		t.Fatal(m.Error)
	}
	if err := call(id); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
	// A concurrent connection cannot steal this session's MCP lease.
	q := startPeer(t, c)
	m := q.call("session/load", map[string]any{"sessionId": id, "cwd": cwd, "mcpServers": servers})
	if m.Error == nil {
		t.Fatal("concurrent owner accepted")
	}
	if err := call(id); err != nil {
		t.Fatal("owner disrupted", err)
	}
	_ = p.in.Close()
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup timed out")
	}
	if err := call(id); err == nil {
		t.Fatal("MCP survived disconnect")
	}
	// Credentials/config are not recovered from persistent history.
	result(t, q.call("session/load", map[string]any{"sessionId": id, "cwd": cwd, "mcpServers": []any{}}))
	if err := call(id); err == nil {
		t.Fatal("ephemeral config recovered from storage")
	}
}

func TestMCPInvalidAndFailedSetupIsRedacted(t *testing.T) {
	c, _ := daemonRig(t, &gated{})
	p := startPeer(t, c)
	for _, server := range []any{
		map[string]any{"name": "bad", "command": "relative", "args": []string{}, "env": []any{}},
		map[string]any{"type": "unknown", "name": "bad", "url": "http://sandbox-secret"},
		map[string]any{"type": "http", "name": "bad", "url": "http://127.0.0.1:1/sandbox-secret", "headers": []any{}},
	} {
		m := p.call("session/new", map[string]any{"cwd": t.TempDir(), "mcpServers": []any{server}})
		if m.Error == nil {
			t.Fatal("invalid/setup failure accepted")
		}
		if strings.Contains(string(mustJSON(m)), "sandbox-secret") {
			t.Fatal("credential leaked")
		}
	}
}

func TestMCPStdioSandboxAndEOF(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c, _ := daemonRig(t, &gated{})
	p := startPeer(t, c)
	cwd := t.TempDir()
	servers := []any{map[string]any{"name": "stdio-tools", "command": executable, "args": []string{"-test.run=^TestACPStdioHelper$"}, "env": []any{map[string]string{"name": "MENO_ACP_TEST_HELPER", "value": "1"}, map[string]string{"name": "MENO_ACP_TEST_CWD", "value": cwd}}}}
	id := result(t, p.call("session/new", map[string]any{"cwd": cwd, "mcpServers": servers}))["sessionId"].(string)
	r, err := c.Resource(context.Background(), "POST", "/v1/sessions/"+id+"/mcp/call", "", api.ResourceRequest{Data: json.RawMessage(`{"server":"stdio-tools","tool":"echo","args":{}}`)})
	if err != nil || !strings.Contains(string(r.Data), "stdio-sandbox") {
		t.Fatalf("stdio call: %s %v", r.Data, err)
	}
	_ = p.in.Close()
	<-p.done
	if _, err = c.Resource(context.Background(), "GET", "/v1/sessions/"+id+"/mcp/tools/stdio-tools", "", api.ResourceRequest{}); err != nil {
		t.Fatal(err)
	}
}

func TestACPStdioHelper(t *testing.T) {
	if os.Getenv("MENO_ACP_TEST_HELPER") != "1" {
		return
	}
	cwd, _ := os.Getwd()
	expected, _ := filepath.EvalSymlinks(os.Getenv("MENO_ACP_TEST_CWD"))
	if cwd != expected {
		os.Exit(3)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var q struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(scanner.Bytes(), &q) != nil || len(q.ID) == 0 {
			continue
		}
		var res any
		switch q.Method {
		case "initialize":
			res = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "stdio", "version": "1"}}
		case "tools/list":
			res = map[string]any{"tools": []any{map[string]any{"name": "echo", "inputSchema": map[string]string{"type": "object"}}}}
		case "tools/call":
			res = map[string]any{"content": []any{map[string]string{"type": "text", "text": "stdio-sandbox"}}}
		default:
			fmt.Fprintf(os.Stdout, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32601,"message":"unsupported"}}`+"\n", q.ID)
			continue
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": res})
	}
	os.Exit(0)
}

func TestMCPSSESandbox(t *testing.T) {
	events := make(chan []byte, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sse" {
			w.Header().Set("Content-Type", "text/event-stream")
			f := w.(http.Flusher)
			fmt.Fprint(w, "event: endpoint\ndata: /messages\n\n")
			f.Flush()
			for {
				select {
				case data := <-events:
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
					f.Flush()
				case <-r.Context().Done():
					return
				}
			}
		}
		var q struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		w.WriteHeader(202)
		if len(q.ID) == 0 {
			return
		}
		var res any
		switch q.Method {
		case "initialize":
			res = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "serverInfo": map[string]string{"name": "sse", "version": "1"}}
		case "tools/list":
			res = map[string]any{"tools": []any{map[string]any{"name": "echo", "inputSchema": map[string]string{"type": "object"}}}}
		case "tools/call":
			res = map[string]any{"content": []any{map[string]string{"type": "text", "text": "sse-sandbox"}}}
		}
		data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": res})
		events <- data
	}))
	t.Cleanup(server.Close)
	c, _ := daemonRig(t, &gated{})
	p := startPeer(t, c)
	servers := []any{map[string]any{"type": "sse", "name": "sse-tools", "url": server.URL + "/sse", "headers": []any{}}}
	id := result(t, p.call("session/new", map[string]any{"cwd": t.TempDir(), "mcpServers": servers}))["sessionId"].(string)
	r, err := c.Resource(context.Background(), "POST", "/v1/sessions/"+id+"/mcp/call", "", api.ResourceRequest{Data: json.RawMessage(`{"server":"sse-tools","tool":"echo","args":{}}`)})
	if err != nil || !strings.Contains(string(r.Data), "sse-sandbox") {
		t.Fatalf("SSE call: %s %v", r.Data, err)
	}
	_ = p.in.Close()
	<-p.done
}
