package daemonapi_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"meno/internal/client"
	"meno/internal/proto"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMCPRuntimeAndResultsThroughDaemon(t *testing.T) {
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
		var result any
		switch req.Method {
		case "initialize":
			starts.Add(1)
			result = map[string]any{"protocolVersion": "2025-06-18"}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "echo", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "retained echo"}, map[string]any{"type": "image", "data": base64.StdEncoding.EncodeToString([]byte("PNG")), "mimeType": "image/png"}}, "structuredContent": map[string]any{"value": 42}, "isError": true}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer fake.Close()
	home, eng, sess, api, socket := startRig(t, &scripted{})
	cl := client.New(home)
	cl.Socket = socket
	server := proto.ConfigMCPServer{Name: "pencil", Transport: "streamable-http", URL: fake.URL}
	if _, err := cl.AddMcpServer(context.Background(), server); err != nil {
		t.Fatal(err)
	}
	server.Enabled = true
	if _, err := cl.UpdateMcpServer(context.Background(), server.Name, server); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.UpdateMcpServer(context.Background(), server.Name, server); err != nil {
		t.Fatal(err)
	}
	servers, err := api.MCPServers()
	if err != nil || len(servers) != 1 || servers[0].Status != "running" || starts.Load() != 1 {
		t.Fatalf("runtime %+v starts=%d err=%v", servers, starts.Load(), err)
	}
	tools, err := api.MCPTools("pencil")
	if err != nil || len(tools) != 1 {
		t.Fatalf("tools %+v %v", tools, err)
	}
	res, err := api.MCPCall("pencil", "echo", map[string]any{})
	if err != nil || !res.IsError || res.Text != "retained echo" || len(res.Artifacts) != 1 || string(res.StructuredContent) != `{"value":42}` {
		t.Fatalf("result %+v %v", res, err)
	}
	if b, err := os.ReadFile(res.Artifacts[0].Path); err != nil || string(b) != "PNG" {
		t.Fatalf("artifact %q %v", b, err)
	}
	page, err := api.MCPResults(res.ResultID, 0, 0)
	if err != nil || !strings.Contains(page.Text, "retained echo") || !strings.Contains(page.Text, "artifact-1.png") {
		t.Fatalf("stored %+v %v", page, err)
	}
	code, out, errs := run(t, api, "mcp", "call", "pencil", "echo")
	if code != 0 || !strings.Contains(out, `"isError":true`) || !strings.Contains(out, `"resultId"`) {
		t.Fatalf("CLI %d %s %s", code, out, errs)
	}
	server.Enabled = false
	if _, err = cl.UpdateMcpServer(context.Background(), server.Name, server); err != nil {
		t.Fatal(err)
	}
	if _, err = api.MCPCall("pencil", "echo", nil); err == nil {
		t.Fatal("disabled server callable")
	}
	if err = eng.DeleteSession(context.Background(), sess.Session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(res.Artifacts[0].Path); !os.IsNotExist(err) {
		t.Fatalf("session artifact not removed: %v", err)
	}
	server.Enabled = true
	if _, err = cl.UpdateMcpServer(context.Background(), server.Name, server); err != nil {
		t.Fatal(err)
	}
	// Engine shutdown cannot own/close the shared MCP runtime.
	eng.Close()
	status, err := cl.McpServers(context.Background())
	if err != nil || !status.Servers[0].Running {
		t.Fatalf("engine closed shared runtime: %+v %v", status, err)
	}

}

// An agent that recovers a broken MCP server must not wait for a question:
// the restart runs without a confirmation, or the run stalls until it times out.
func TestMCPRestartRunsWithoutAQuestion(t *testing.T) {
	home, eng, sess, api, socket := startRig(t, &scripted{})
	cl := client.New(home)
	cl.Socket = socket
	script := filepath.Join(t.TempDir(), "server.py")
	body := "import sys, json\n" +
		"for line in sys.stdin:\n" +
		"    m = json.loads(line)\n" +
		"    if 'id' not in m: continue\n" +
		"    meth = m.get('method')\n" +
		"    if meth == 'initialize': r = {'protocolVersion': '2025-06-18', 'capabilities': {}, 'serverInfo': {'name': 'inv', 'version': '1'}}\n" +
		"    elif meth == 'tools/list': r = {'tools': [{'name': 'list'}]}\n" +
		"    else: r = {}\n" +
		"    sys.stdout.write(json.dumps({'jsonrpc': '2.0', 'id': m['id'], 'result': r}) + '\\n')\n" +
		"    sys.stdout.flush()\n"
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	server := proto.ConfigMCPServer{Name: "inventory", Transport: "stdio", Command: "python3", Args: []string{script}, Enabled: true}
	if _, err := cl.AddMcpServer(context.Background(), server); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := api.MCPAdmin("restart", server.Name); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("restart: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("restart waited for a question")
	}
	snapshot, err := eng.Snapshot(context.Background(), sess.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Ask != nil {
		t.Fatalf("the restart opened a question: %+v", snapshot.Ask)
	}
	if snapshot.Phase == "asking" {
		t.Fatal("the session is waiting for an answer")
	}
}

func TestMCPAdministrationRequiresApproval(t *testing.T) {
	home, eng, sess, api, socket := startRig(t, &scripted{})
	cl := client.New(home)
	cl.Socket = socket
	server := proto.ConfigMCPServer{Name: "configured", Transport: "stdio", Command: "true"}
	if _, err := cl.AddMcpServer(context.Background(), server); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := api.MCPAdmin("remove", server.Name); done <- err }()
	var ask *proto.AskForm
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		snapshot, err := eng.Snapshot(context.Background(), sess.Session.ID)
		if err != nil {
			t.Fatal(err)
		}
		ask = snapshot.Ask
		if ask != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if ask == nil {
		t.Fatal("confirmation never opened")
	}
	if err := eng.AnswerAsk(context.Background(), sess.Session.ID, ask.AskID, proto.AskAnswerRequest{Answers: map[string]json.RawMessage{"approve": json.RawMessage(`["yes"]`)}}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("confirmation stuck")
	}
	all, err := cl.McpServers(context.Background())
	if err != nil || len(all.Servers) != 0 {
		t.Fatalf("remove %+v %v", all, err)
	}
	// A token cannot skip confirmation through the global configuration endpoint.
	_, err = cl.Resource(context.Background(), "POST", "/v1/mcp/servers", "", proto.ResourceRequest{Token: sess.AgentToken})
	if err == nil {
		t.Fatal("agent token bypassed human route")
	}
}
