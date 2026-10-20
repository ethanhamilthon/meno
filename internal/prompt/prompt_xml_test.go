package prompt

import (
	"strings"
	"testing"
)

// The exact prompt of a fixed input. It guards the format: if "## " headings
// come back, or an element changes, this test fails.
func TestSubagentPromptHasRoleAndNoAskRule(t *testing.T) {
	in := full()
	in.Subagent = true
	in.Allow = []string{"agents", "goal", "help"}
	out := Build(in)
	if !strings.Contains(out, "<subagent-role>\nA parent agent started you") || !strings.Contains(out, "</subagent-role>") {
		t.Fatalf("no subagent role:\n%s", out)
	}
	if strings.Contains(out, "meno ask") {
		t.Fatalf("a subagent has no ask rule:\n%s", out)
	}
	if strings.Index(out, "</guidelines>") > strings.Index(out, "<subagent-role>") ||
		strings.Index(out, "</subagent-role>") > strings.Index(out, "<skills>") {
		t.Fatalf("the role goes between guidelines and skills:\n%s", out)
	}
	if !strings.Contains(out, "<method>") {
		t.Fatalf("subagent policy must keep method rules:\n%s", out)
	}
	for _, name := range []string{"editing", "verification", "output"} {
		if strings.Contains(out, "<"+name+">") {
			t.Errorf("subagent policy must omit <%s>", name)
		}
	}
}

func TestNoHeadingLinesAndEveryElementOpensOnce(t *testing.T) {
	for _, sub := range []bool{false, true} {
		in := full()
		in.Subagent = sub
		out := Build(in)
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "## ") {
				t.Fatalf("heading line %q (subagent=%v)", l, sub)
			}
		}
		for _, name := range []string{"voice", "method", "guidelines", "skills", "global-instructions", "project-instructions", "mcp-servers", "environment"} {
			if strings.Count(out, "<"+name+">") != 1 || strings.Count(out, "</"+name+">") != 1 {
				t.Fatalf("<%s> must open and close once (subagent=%v):\n%s", name, sub, out)
			}
		}
		for _, name := range []string{"editing", "verification", "output"} {
			if strings.Count(out, "<"+name+">") != 1 || strings.Count(out, "</"+name+">") != 1 {
				t.Fatalf("<%s> must open and close once in an unrestricted prompt:\n%s", name, out)
			}
		}
	}
}
