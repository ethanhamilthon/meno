package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// fakeHTTP serves a streamable-http server whose sessions can be expired.
type fakeHTTP struct {
	mu       sync.Mutex
	session  int
	valid    string
	inits    int
	methods  []string
	initResp func() map[string]any
}

func (f *fakeHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req rpcReq
	b, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(b, &req)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.methods = append(f.methods, req.Method)
	if req.Method != "initialize" && r.Header.Get("Mcp-Session-Id") != f.valid {
		http.Error(w, "unknown session", http.StatusNotFound)
		return
	}
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	result := reply(req.Method, req.Params)
	if req.Method == "initialize" {
		f.inits++
		f.session++
		f.valid = "sess-" + string(rune('0'+f.session))
		w.Header().Set("Mcp-Session-Id", f.valid)
		if f.initResp != nil {
			result = f.initResp()
		}
	}
	msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": result})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(msg)
}

func TestExpiredHTTPSessionIsReinitialized(t *testing.T) {
	f := &fakeHTTP{}
	srv := httptest.NewServer(f)
	defer srv.Close()
	c, err := Start(context.Background(), Server{Name: "r", Transport: TransportHTTP, URL: srv.URL, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	f.mu.Lock()
	f.valid = "forgotten" // the server lost the session
	f.mu.Unlock()
	out, err := c.CallResult(context.Background(), "echo", map[string]any{"msg": "hi"})
	if err != nil || out.Text != "echo:hi" {
		t.Fatalf("call after expiry: %+v %v", out, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inits != 2 {
		t.Fatalf("want one re-initialize, got %d inits: %v", f.inits, f.methods)
	}
}

func TestOnlyResourcesServerStartsWithoutToolsList(t *testing.T) {
	f := &fakeHTTP{initResp: func() map[string]any {
		return map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{"resources": map[string]any{}, "prompts": map[string]any{}}}
	}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	c, err := Start(context.Background(), Server{Name: "r", Transport: TransportHTTP, URL: srv.URL, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if len(c.ToolSnapshot()) != 0 || c.Capabilities().Resources == nil || c.Capabilities().Tools != nil {
		t.Fatalf("caps %+v tools %v", c.Capabilities(), c.ToolSnapshot())
	}
	for _, m := range f.methods {
		if m == "tools/list" {
			t.Fatal("tools/list must not be sent without a tools capability")
		}
	}
}

func TestUndeclaredCapabilitiesStillListTools(t *testing.T) {
	f := &fakeHTTP{}
	srv := httptest.NewServer(f)
	defer srv.Close()
	c, err := Start(context.Background(), Server{Name: "r", Transport: TransportHTTP, URL: srv.URL, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if len(c.ToolSnapshot()) != 1 {
		t.Fatalf("tools %v", c.ToolSnapshot())
	}
}

func TestToolMetadataIsRetained(t *testing.T) {
	var tool Tool
	raw := `{"name":"t","title":"T","description":"d","inputSchema":{"type":"object"},"outputSchema":{"type":"object"},"annotations":{"readOnlyHint":true},"_meta":{"k":1}}`
	if err := json.Unmarshal([]byte(raw), &tool); err != nil {
		t.Fatal(err)
	}
	if tool.Title != "T" || string(tool.OutputSchema) != `{"type":"object"}` || string(tool.Annotations) != `{"readOnlyHint":true}` || string(tool.Meta) != `{"k":1}` {
		t.Fatalf("%+v", tool)
	}
}
