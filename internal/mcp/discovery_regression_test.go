package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestStatelessNegotiatedVersionAndNotifications(t *testing.T) {
	var lists atomic.Int32
	var badHeader atomic.Bool
	var initialized atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method != "initialize" && r.Header.Get("MCP-Protocol-Version") != "2025-03-26" {
			badHeader.Store(true)
		}
		if req.ID == nil {
			initialized.Store(true)
			w.WriteHeader(202)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-03-26"}
		case "tools/list":
			if !initialized.Load() {
				t.Error("tools/list arrived before initialized")
			}
			n := lists.Add(1)
			name := "first"
			if n > 1 {
				name = "changed"
			}
			result = map[string]any{"tools": []any{map[string]any{"name": name}}}
		case "tools/call":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/tools/list_changed\"}\n\n")
			b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"content": []any{}}})
			fmt.Fprintf(w, "data: %s\n\n", b)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := Start(ctx, Server{Name: "stateless", Transport: TransportHTTP, URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err = c.CallResult(ctx, "first", nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		tools := c.ToolSnapshot()
		if len(tools) == 1 && tools[0].Name == "changed" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if tools := c.ToolSnapshot(); len(tools) != 1 || tools[0].Name != "changed" {
		t.Fatalf("stale tools %+v", tools)
	}
	if badHeader.Load() {
		t.Fatal("missing negotiated stateless header")
	}
	cancel()
	select {
	case <-c.done:
	case <-time.After(time.Second):
		t.Fatal("canceled lifetime stayed alive")
	}
}

func TestSSERejectsCrossOriginEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: endpoint\ndata: https://other.example/messages\n\n")
	}))
	defer srv.Close()
	if c, err := Start(context.Background(), Server{Name: "cross", Transport: TransportSSE, URL: srv.URL}); err == nil {
		c.Close()
		t.Fatal("cross-origin endpoint accepted")
	}
}

func TestConcurrentRemoteCloseAndEOF(t *testing.T) {
	for i := 0; i < 50; i++ {
		_, c := newRemote(context.Background(), Server{})
		done := make(chan struct{})
		go func() { c.Close(); close(done) }()
		c.markDone()
		c.Close()
		<-done
	}
}
