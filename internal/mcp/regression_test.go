package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStoredTextFallback(t *testing.T) {
	s := ResultStore{Dir: t.TempDir()}
	st, err := s.Save("session", "server", "echo", Result{Text: "retained echo"})
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.Read("session", st.ID, 0, 0)
	if err != nil || !strings.Contains(page.Text, "retained echo") {
		t.Fatalf("lost text: %+v %v", page, err)
	}
}

func TestToolsPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     *int64         `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			w.WriteHeader(202)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": protocolVersion}
		case "tools/list":
			if req.Params["cursor"] == "two" {
				result = map[string]any{"tools": []any{map[string]any{"name": "second"}}}
			} else {
				result = map[string]any{"tools": []any{map[string]any{"name": "first"}}, "nextCursor": "two"}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer srv.Close()
	c, err := Start(context.Background(), Server{Name: "pages", Transport: TransportHTTP, URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if len(c.ToolSnapshot()) != 2 {
		t.Fatalf("lost tool pages: %+v", c.ToolSnapshot())
	}
}

func TestStoreMetadataAndSmallUnicodePage(t *testing.T) {
	s := ResultStore{Dir: t.TempDir()}
	raw := json.RawMessage(`{"content":[{"type":"text","text":"界"}],"_meta":{"kept":true}}`)
	result, err := ParseResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.Save("session", "server", "tool", result)
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.Info("session", stored.ID)
	if err != nil || info.Server != "server" || info.Tool != "tool" {
		t.Fatalf("metadata %+v %v", info, err)
	}
	page, err := s.Read("session", stored.ID, 0, 0)
	if err != nil || !strings.Contains(page.Text, `"_meta"`) {
		t.Fatalf("lost unknown fields %+v %v", page, err)
	}
	offset := strings.Index(page.Text, "界")
	if _, err = s.Read("session", stored.ID, offset, 1); err == nil {
		t.Fatal("no-progress Unicode page accepted")
	}
}
