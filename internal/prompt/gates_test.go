package prompt

import (
	"strings"
	"testing"
)

func TestRestrictedGroupsDropFileSections(t *testing.T) {
	out := Build(Input{Cwd: "/w", Allow: []string{"ask", "goal", "help"}})
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
