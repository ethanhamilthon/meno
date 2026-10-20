package prompt

import (
	"strings"
	"testing"
)

func TestRestrictedGroupsDropFileSections(t *testing.T) {
	out := Build(Input{Cwd: "/w", Allow: []string{"agents", "ask", "goal", "help"}})
	for _, name := range []string{"editing", "verification", "output"} {
		if strings.Contains(out, "<"+name+">") {
			t.Errorf("restricted prompt includes <%s>", name)
		}
	}
	for _, rule := range []string{"meno read", "meno mcp", "meno tasks", "meno web"} {
		if strings.Contains(out, rule) {
			t.Errorf("restricted method includes %q", rule)
		}
	}
	if !strings.Contains(out, "meno agents") {
		t.Fatalf("an allowed group is missing from the method:\n%s", out)
	}
}

func TestNilAllowKeepsAllSections(t *testing.T) {
	out := Build(Input{Cwd: "/w"})
	for _, name := range []string{"voice", "method", "editing", "verification", "output", "guidelines"} {
		if strings.Count(out, "<"+name+">") != 1 {
			t.Errorf("an unrestricted prompt must include <%s> once", name)
		}
	}
	if !strings.Contains(out, "meno ask") {
		t.Fatal("main prompt must keep its ask rule")
	}
}

func TestSubagentKeepsRoleButNoAskRule(t *testing.T) {
	out := Build(Input{Cwd: "/w", Subagent: true, Allow: []string{"files", "mcp", "agents", "web", "view", "models", "help"}})
	if !strings.Contains(out, "<subagent-role>") {
		t.Fatalf("child prompt must keep its role:\n%s", out)
	}
	if strings.Contains(out, "meno ask") {
		t.Fatalf("child prompt must not include ask instructions:\n%s", out)
	}
}
