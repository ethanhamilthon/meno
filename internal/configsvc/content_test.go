package configsvc

import (
	"context"
	"encoding/json"
	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/mcp"
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
	call := func(method, path, data string) api.ResourceResponse {
		t.Helper()
		r, e := s.Resource(context.Background(), method, path, "", api.ResourceRequest{Data: json.RawMessage(data)})
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

func TestSkillsAreListedFromTheAskingDirectory(t *testing.T) {
	p := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	project := t.TempDir()
	dir := filepath.Join(project, ".meno", "skills", "local")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: local\ndescription: project only\n---\nBody."), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(Options{Paths: p, MCP: mcp.NewManager()})
	if err != nil {
		t.Fatal(err)
	}
	list := func(query string) []api.ConfigSkill {
		t.Helper()
		r, err := s.Resource(context.Background(), "GET", "/v1/skills", query, api.ResourceRequest{})
		if err != nil {
			t.Fatal(err)
		}
		var out api.SkillsResponse
		if err = json.Unmarshal(r.Data, &out); err != nil {
			t.Fatal(err)
		}
		return out.Skills
	}
	if got := list(""); len(got) != 0 {
		t.Fatalf("the daemon directory has no skills: %+v", got)
	}
	if got := list("cwd=" + project); len(got) != 1 || got[0].Name != "local" || got[0].Auto {
		t.Fatalf("project skill: %+v", got)
	}
	on := true
	body, _ := json.Marshal(api.SkillEnabledRequest{Enabled: true, Auto: &on, Cwd: project})
	if _, err = s.Resource(context.Background(), "PUT", "/v1/skills/local/enabled", "", api.ResourceRequest{Data: body}); err != nil {
		t.Fatal(err)
	}
	if got := list("cwd=" + project); len(got) != 1 || !got[0].Auto || !got[0].Enabled {
		t.Fatalf("auto flag not saved: %+v", got)
	}
}
