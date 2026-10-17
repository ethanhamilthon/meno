package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// reply builds the result for a JSON-RPC request of the fake server.
func reply(method string, params json.RawMessage) any {
	switch method {
	case "initialize":
		return map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "fake"}}
	case "tools/list":
		return map[string]any{"tools": []map[string]any{{"name": "echo", "description": "echo", "inputSchema": map[string]any{"type": "object"}}}}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(params, &p)
		return map[string]any{"content": []map[string]any{{"type": "text", "text": fmt.Sprint("echo:", p.Arguments["msg"])}}}
	}
	return map[string]any{}
}

type rpcReq struct {
	ID     *int64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func TestStreamableHTTP(t *testing.T) {
	var mu sync.Mutex
	var sessions, auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcReq
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &req)
		mu.Lock()
		sessions = append(sessions, req.Method+"="+r.Header.Get("Mcp-Session-Id"))
		auths = append(auths, r.Header.Get("Authorization"))
		mu.Unlock()
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": reply(req.Method, req.Params)})
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "sess-1")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(msg)
		case "tools/list":
			// streamed answer, with a notification in front of it
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\"}\n\nevent: message\ndata: %s\n\n", msg)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(msg)
		}
	}))
	defer srv.Close()
	c, err := Start(context.Background(), Server{Name: "remote", Transport: TransportHTTP, URL: srv.URL, Headers: map[string]string{"Authorization": "Bearer k"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if len(c.Tools) != 1 || c.Tools[0].Name != "echo" {
		t.Fatalf("tools: %+v", c.Tools)
	}
	out, err := c.Call(context.Background(), "echo", map[string]any{"msg": "hi"})
	if err != nil || out != "echo:hi" {
		t.Fatalf("call: %q %v", out, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if sessions[0] != "initialize=" || !strings.Contains(strings.Join(sessions[1:], " "), "tools/call=sess-1") {
		t.Fatalf("session id not echoed back: %v", sessions)
	}
	for _, a := range auths {
		if a != "Bearer k" {
			t.Fatalf("custom header missing: %v", auths)
		}
	}
}

func TestStreamableHTTPErrorsSurface(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := Start(context.Background(), Server{Name: "x", Transport: TransportHTTP, URL: srv.URL, Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("error: %v", err)
	}
}

func TestLegacySSE(t *testing.T) {
	events := make(chan string, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sse":
			w.Header().Set("Content-Type", "text/event-stream")
			fl := w.(http.Flusher)
			_, _ = fmt.Fprint(w, "event: endpoint\ndata: /messages?session=7\n\n")
			fl.Flush()
			for {
				select {
				case m := <-events:
					_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", m)
					fl.Flush()
				case <-r.Context().Done():
					return
				}
			}
		case "/messages":
			if r.URL.Query().Get("session") != "7" {
				http.Error(w, "bad session", http.StatusBadRequest)
				return
			}
			var req rpcReq
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &req)
			w.WriteHeader(http.StatusAccepted)
			if req.ID != nil {
				msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": reply(req.Method, req.Params)})
				events <- string(msg)
			}
		}
	}))
	defer srv.Close()
	c, err := Start(context.Background(), Server{Name: "legacy", Transport: TransportSSE, URL: srv.URL + "/sse", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Tools) != 1 {
		t.Fatalf("tools: %+v", c.Tools)
	}
	out, err := c.Call(context.Background(), "echo", map[string]any{"msg": "sse"})
	if err != nil || out != "echo:sse" {
		t.Fatalf("call: %q %v", out, err)
	}
	if !c.Alive() {
		t.Fatal("client should be alive")
	}
	c.Close()
	deadline := time.Now().Add(2 * time.Second)
	for c.Alive() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if c.Alive() {
		t.Fatal("client alive after Close")
	}
}

func TestValidateRemoteURL(t *testing.T) {
	for _, u := range []string{"localhost:9100", "ftp://x/y", "http://", "not a url"} {
		if err := ValidateServer(Server{Name: "x", Transport: TransportSSE, URL: u}); err == nil {
			t.Errorf("%q accepted", u)
		}
	}
	if err := ValidateServer(Server{Name: "x", Transport: TransportHTTP, URL: "https://example.com/mcp"}); err != nil {
		t.Fatal(err)
	}
}
