package configsvc

import (
	"context"
	"encoding/json"
	"meno/internal/api"
	"meno/internal/config"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestMCPFileRestartAndReadOnlyDefinition(t *testing.T) {
	var starts atomic.Int32
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			w.WriteHeader(202)
			return
		}
		var result any = map[string]any{"tools": []any{}}
		if req.Method == "initialize" {
			starts.Add(1)
			result = map[string]any{"protocolVersion": "2025-06-18"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer fake.Close()
	paths := config.Paths{Home: t.TempDir()}
	data, _ := json.Marshal([]api.ConfigMCPServer{{Name: "file-server", Transport: "streamable-http", URL: fake.URL, Enabled: true}})
	if err := os.WriteFile(paths.MCPFile(), data, 0600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(Options{Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	ctx := context.Background()
	path := "/v1/mcp/servers/file-server"
	if _, err = svc.Resource(ctx, "GET", path, "", api.ResourceRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Resource(ctx, "POST", path+"/restart", "", api.ResourceRequest{}); err != nil {
		t.Fatal(err)
	}
	// The restart runs in the background: the handshake must not hold the
	// configuration lock.
	for deadline := time.Now().Add(5 * time.Second); starts.Load() == 0 && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
	}
	if starts.Load() != 1 {
		t.Fatalf("starts=%d", starts.Load())
	}
	if _, err = svc.Resource(ctx, "DELETE", path, "", api.ResourceRequest{}); err == nil {
		t.Fatal("file definition writable")
	}
}

func TestMCPApprovalPreconditionAndWarnings(t *testing.T) {
	paths := config.Paths{Home: t.TempDir()}
	svc, err := New(Options{Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	server := api.ConfigMCPServer{Name: "saved", Transport: "stdio", Command: "true"}
	data, _ := json.Marshal(api.McpServerRequest{Server: server})
	if _, err = svc.Resource(context.Background(), "POST", "/v1/mcp/servers", "", api.ResourceRequest{Data: data}); err != nil {
		t.Fatal(err)
	}
	expected := server
	expected.Command = "changed"
	if _, err = svc.Resource(context.Background(), "DELETE", "/v1/mcp/servers/saved", "", api.ResourceRequest{ExpectedMCP: &expected}); err == nil {
		t.Fatal("stale approval accepted")
	}
	if err = os.WriteFile(paths.MCPFile(), []byte("broken JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	response, err := svc.Resource(context.Background(), "GET", "/v1/mcp/servers", "", api.ResourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var out api.McpServersResponse
	_ = json.Unmarshal(response.Data, &out)
	if len(out.Warnings) == 0 || len(out.Servers) != 1 {
		t.Fatalf("lost config/warnings %+v", out)
	}
}
