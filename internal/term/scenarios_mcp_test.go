//go:build termtest

package term

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMCPRuntimeSharedWithAgentAndDialog(t *testing.T) {
	var starts atomic.Int32
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			w.WriteHeader(202)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			starts.Add(1)
			result = map[string]any{"protocolVersion": "2025-06-18"}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "echo", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "mcp-result-marker"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer mcp.Close()
	home := t.TempDir()
	data, _ := json.Marshal([]map[string]any{{"name": "pencil", "transport": "streamable-http", "url": mcp.URL}})
	if err := os.WriteFile(filepath.Join(home, "mcp.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		if q.Mentions("mcp-result-marker") {
			q.Text("MCP-connected")
		} else {
			q.Tool("", "meno mcp call pencil echo '{}'")
		}
	}
	s := start(t, options{fake: f, home: home})
	s.Cmd("mcp")
	s.WaitFor("streamable-http · running")
	if starts.Load() != 1 {
		t.Fatalf("duplicate MCP initialization: %d", starts.Load())
	}
	s.Send("r")
	deadline := time.Now().Add(3 * time.Second)
	for starts.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if starts.Load() != 2 {
		t.Fatalf("file restart did not initialize once: %d", starts.Load())
	}
	s.Send(Esc)
	s.WaitGone("mcp servers")
	s.Say("test MCP")
	screen := s.WaitFor("MCP-connected")
	if !strings.Contains(screen, "mcp-result-marker") {
		t.Fatal("MCP result missing from terminal")
	}
}
