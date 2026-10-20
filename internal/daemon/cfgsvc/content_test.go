package cfgsvc

import (
	"context"
	"encoding/json"
	"meno/internal/config"
	"meno/internal/mcp"
	"meno/internal/proto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsAndMCPResources(t *testing.T) {
	p := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	_ = os.MkdirAll(filepath.Join(p.GlobalSkillsDir(), "review"), 0700)
	if e := os.WriteFile(filepath.Join(p.GlobalSkillsDir(), "review", "SKILL.md"), []byte("---\nname: review\ndescription: Review code\n---\nDo a review."), 0600); e != nil {
		t.Fatal(e)
	}
	s, e := New(Options{Paths: p, MCP: mcp.NewManager()})
	if e != nil {
		t.Fatal(e)
	}
	call := func(method, path, data string) proto.ResourceResponse {
		t.Helper()
		r, e := s.Resource(context.Background(), method, path, "", proto.ResourceRequest{Data: json.RawMessage(data)})
		if e != nil {
			t.Fatalf("%s %s: %v", method, path, e)
		}
		return r
	}
	if r := call("GET", "/v1/skills", ""); !strings.Contains(string(r.Data), "review") {
		t.Fatalf("skills %s", r.Data)
	}
	if r := call("PUT", "/v1/skills/review/enabled", "{\"enabled\":false}"); !strings.Contains(string(r.Data), "\"enabled\":false") {
		t.Fatalf("skill %s", r.Data)
	}
	if r := call("POST", "/v1/mcp/servers", "{\"server\":{\"name\":\"local\",\"transport\":\"stdio\",\"command\":\"true\",\"enabled\":false}}"); !strings.Contains(string(r.Data), "local") {
		t.Fatalf("MCP create %s", r.Data)
	}
	if r := call("PUT", "/v1/mcp/servers/local", "{\"server\":{\"name\":\"local\",\"transport\":\"stdio\",\"command\":\"true\",\"enabled\":false}}"); !strings.Contains(string(r.Data), "local") {
		t.Fatalf("MCP update %s", r.Data)
	}
	call("DELETE", "/v1/mcp/servers/local", "")
}
