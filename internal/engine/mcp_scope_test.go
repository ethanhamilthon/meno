package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/mcp"
)

func scopeSandbox(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		var q struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		if len(q.ID) == 0 {
			w.WriteHeader(202)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch q.Method {
		case "initialize":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"fake","version":"1"}}}`, q.ID)
		case "tools/list":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"tools":[{"name":"echo","inputSchema":{"type":"object"}}]}}`, q.ID)
		case "tools/call":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"echo"}]}}`, q.ID)
		default:
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32601,"message":"unsupported"}}`, q.ID)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
func scopeInput(owner, url, name string) api.ResourceRequest {
	return api.ResourceRequest{Data: json.RawMessage(fmt.Sprintf(`{"owner":%q,"servers":[{"type":"http","name":%q,"url":%q,"headers":[]}]}`, owner, name, url))}
}
func TestMCPScopeGlobalPreservationRollbackAndLifecycle(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	url := scopeSandbox(t)
	if err := e.mcp.Start(context.Background(), mcp.Server{Name: "global", Transport: mcp.TransportHTTP, URL: url, Protocol: mcp.ProtocolLegacy}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.mcp.StopAll)
	owner := "owner-for-test-123"
	q := scopeInput(owner, url, "client")
	if _, err := e.scopeResource(context.Background(), s, "PUT", q); err != nil {
		t.Fatal(err)
	}
	original := e.scopedManager(s)
	if _, err := e.scopeResource(context.Background(), s, "PUT", scopeInput(owner, url, "global")); err == nil {
		t.Fatal("global collision accepted")
	}
	failed := api.ResourceRequest{Data: json.RawMessage(fmt.Sprintf(`{"owner":%q,"servers":[{"type":"http","name":"first","url":%q,"headers":[]},{"type":"http","name":"failed","url":"http://127.0.0.1:1","headers":[]}]}`, owner, url))}
	if _, err := e.scopeResource(context.Background(), s, "PUT", failed); err == nil {
		t.Fatal("partial failure accepted")
	}
	if original != e.scopedManager(s) || !original.Running("client") || !e.mcp.Running("global") {
		t.Fatal("rollback/global state damaged")
	}
	system, _, _ := e.prepare(context.Background(), s, "", true)
	if !strings.Contains(system, "client") || !strings.Contains(system, "global") {
		t.Fatal("MCP tools missing from prompt")
	}
	// A token-bearing agent cannot mutate the lease, preserving existing policy.
	forbidden := q
	forbidden.Token = "agent-token"
	if _, err := e.scopeResource(context.Background(), s, "DELETE", forbidden); err == nil {
		t.Fatal("agent changed client lease")
	}
	if err := e.DeleteSession(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if original.Running("client") || !e.mcp.Running("global") {
		t.Fatal("delete cleanup affected wrong manager")
	}
	id = newSession(t, e)
	s, _ = e.load(context.Background(), id)
	if _, err := e.scopeResource(context.Background(), s, "PUT", q); err != nil {
		t.Fatal(err)
	}
	scoped := e.scopedManager(s)
	e.Close()
	if scoped.Running("client") || !e.mcp.Running("global") {
		t.Fatal("shutdown cleanup affected wrong manager")
	}
}
func TestMCPScopeLeaseExpiryAndHeartbeat(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	q := scopeInput("owner-for-test-123", scopeSandbox(t), "client")
	if _, err := e.scopeResource(context.Background(), s, "PUT", q); err != nil {
		t.Fatal(err)
	}
	s.scopeMu.Lock()
	scope := s.scope
	scope.deadline = time.Now().Add(-time.Second)
	scope.timer.Reset(time.Millisecond)
	s.scopeMu.Unlock()
	deadline := time.Now().Add(2 * time.Second)
	for e.scopedManager(s) != nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if e.scopedManager(s) != nil || scope.manager.Running("client") {
		t.Fatal("orphan lease survived expiry")
	}
	if _, err := e.scopeResource(context.Background(), s, "PATCH", q); err == nil {
		t.Fatal("expired lease heartbeat accepted")
	}
}

func TestMCPScopeCredentialsNotPersistedAndStaleRelease(t *testing.T) {
	e, _ := fixture(t, &fakeProvider{})
	id := newSession(t, e)
	s, _ := e.load(context.Background(), id)
	q := scopeInput("owner-for-test-123", scopeSandbox(t), "client")
	var input map[string]any
	_ = json.Unmarshal(q.Data, &input)
	input["servers"].([]any)[0].(map[string]any)["headers"] = []any{map[string]string{"name": "Authorization", "value": "private-ephemeral-token"}}
	q.Data, _ = json.Marshal(input)
	if _, err := e.scopeResource(context.Background(), s, "PUT", q); err != nil {
		t.Fatal(err)
	}
	input["previous"] = "owner-for-test-123"
	input["owner"] = "replacement-owner-123"
	q.Data, _ = json.Marshal(input)
	if _, err := e.scopeResource(context.Background(), s, "PUT", q); err != nil {
		t.Fatal(err)
	}
	stale := api.ResourceRequest{Data: json.RawMessage(`{"owner":"owner-for-test-123"}`)}
	if _, err := e.scopeResource(context.Background(), s, "DELETE", stale); err == nil {
		t.Fatal("stale owner released replacement")
	}
	if e.scopedManager(s) == nil {
		t.Fatal("replacement lost")
	}
	// Save the session metadata using the normal persistence gate, without a model call.
	s.mu.Lock()
	s.session.Title = "persisted"
	s.persisted = true
	err := e.save(s)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(e.home, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte("private-ephemeral-token")) {
			t.Errorf("ephemeral credential persisted in %s", filepath.Base(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
