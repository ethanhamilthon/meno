package prompt

import (
	"strings"
	"testing"
)

func TestSystem(t *testing.T) {
	s := System("/work")
	if !strings.Contains(s, "/work") || strings.Contains(s, "{{") {
		t.Fatalf("bad system prompt: %q", s)
	}
}

func TestBuildWithoutExtrasIsStable(t *testing.T) {
	// The base prompt plus the fixed mcpServers help is the cache prefix: extras
	// only append to it.
	base := Build("/w", nil, nil)
	if !strings.HasPrefix(base, System("/w")) || !strings.Contains(base, "mcpServers.add(") {
		t.Fatal("base prompt must be the system text followed by the mcpServers help")
	}
	if !strings.HasPrefix(Build("/w", []Skill{{"a", "b", "/p"}}, nil), base) {
		t.Fatal("extras must not change the prefix")
	}
}

func TestBuildSkillsAndTools(t *testing.T) {
	out := Build("/w",
		[]Skill{{"api-security", "check\nauth  and rate limits", "/p/.meno/skills/api-security.md"}, {"bare", "", "/p/bare.md"}},
		[]Tool{{"github", "search-repos", "Search repositories", "{query: string, page?: number}"}})
	for _, want := range []string{
		"## Skills", "- api-security — check auth and rate limits (/p/.meno/skills/api-security.md)", "- bare (/p/bare.md)",
		"## MCP tools", "- mcp.github.search_repos({query: string, page?: number}) — Search repositories", "mcpTools()",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "{{") {
		t.Fatalf("unfilled placeholder:\n%s", out)
	}
}

func TestBuildCapsLongToolLists(t *testing.T) {
	var tools []Tool
	for i := 0; i < maxListedTools+7; i++ {
		tools = append(tools, Tool{Server: "s", Name: "t" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Params: "{}"})
	}
	out := Build("/w", nil, tools)
	if strings.Count(out, "- mcp.s.") != maxListedTools || !strings.Contains(out, "and 7 more") {
		t.Fatalf("not capped:\n%s", out)
	}
}

func TestIdent(t *testing.T) {
	for in, want := range map[string]string{"github": "github", "my-server": "my_server", "9lives": "_lives", "a.b": "a_b", "": "_"} {
		if got := Ident(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
