package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStopSupersedesPendingStart(t *testing.T) {
	entered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
	}))
	defer srv.Close()
	m := NewManager()
	m.StartAsync(context.Background(), Server{Name: "slow", Transport: TransportHTTP, URL: srv.URL})
	if !m.Starting("slow") {
		t.Fatal("start not registered synchronously")
	}
	<-entered
	m.Stop("slow")
	if !m.WaitReady(context.Background(), time.Second) {
		t.Fatal("pending start did not cancel")
	}
	if m.Running("slow") {
		t.Fatal("stopped server published late")
	}
	m.StopAll()
	if err := m.Start(context.Background(), Server{Name: "slow", Transport: TransportHTTP, URL: srv.URL}); err == nil {
		t.Fatal("closed manager restarted")
	}
}

func TestHTTPCallCancellationReachesServer(t *testing.T) {
	canceled := make(chan struct{})
	entered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			w.WriteHeader(202)
			return
		}
		if req.Method == "tools/call" {
			close(entered)
			<-r.Context().Done()
			close(canceled)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": reply(req.Method, req.Params)})
	}))
	defer srv.Close()
	c, err := Start(context.Background(), Server{Name: "cancel", Transport: TransportHTTP, URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := c.CallResult(ctx, "echo", nil); done <- err }()
	<-entered
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("call succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("call stuck")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("HTTP work not canceled")
	}
	if !c.Alive() {
		t.Fatal("one call canceled the connection")
	}
}

func TestRPCStringRequestIDs(t *testing.T) {
	var out recordingWriter
	c := newConn(&out)
	c.handle([]byte(`{"jsonrpc":"2.0","id":"ping-id","method":"ping"}`))
	c.handle([]byte(`{"jsonrpc":"2.0","id":"unknown-id","method":"sampling/createMessage"}`))
	if len(out.messages) != 2 {
		t.Fatalf("responses %s", out.messages)
	}
	var msg map[string]any
	_ = json.Unmarshal(out.messages[1], &msg)
	if msg["id"] != "unknown-id" || msg["error"] == nil {
		t.Fatalf("response %+v", msg)
	}
}

type recordingWriter struct{ messages [][]byte }

func (w *recordingWriter) Write(b []byte) (int, error) {
	w.messages = append(w.messages, append([]byte(nil), b...))
	return len(b), nil
}
