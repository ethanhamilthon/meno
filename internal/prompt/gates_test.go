package prompt

import (
	"strings"
	"testing"
)

func TestPolicyGatesSectionsAndMethodBullets(t *testing.T) {
	in := Input{Cwd: "/w", Only: []string{"agents", "goal"}}
	out := Build(in)
	for _, name := range []string{"editing", "verification", "output"} {
		if strings.Contains(out, "<"+name+">") {
			t.Errorf("restricted prompt includes <%s>", name)
		}
	}
	for _, rule := range []string{"fs.search", "fs.inspect", "fs.patch", "sh(", "tasks.wait"} {
		if strings.Contains(out, rule) {
			t.Errorf("restricted method includes %q", rule)
		}
	}
	if !strings.Contains(out, "- Do independent work") || !strings.Contains(out, "- Do not read a file again") || !strings.Contains(out, "- Stop when the task is done") {
		t.Fatalf("mode-independent method rules are missing:\n%s", out)
	}
}

func TestNilPolicyKeepsAllSections(t *testing.T) {
	out := Build(Input{Cwd: "/w"})
	for _, name := range []string{"voice", "method", "editing", "verification", "output", "guidelines"} {
		if strings.Count(out, "<"+name+">") != 1 {
			t.Errorf("nil policy must include <%s> once", name)
		}
	}
	if !strings.Contains(out, "- Ask: use askUser(") {
		t.Fatal("main prompt must keep its ask rule")
	}
}

func TestSubagentKeepsSectionsButNoAskRule(t *testing.T) {
	in := Input{Cwd: "/w", Subagent: true}
	out := Build(in)
	if !strings.Contains(out, "<subagent-role>") || !strings.Contains(out, "<editing>") {
		t.Fatalf("child prompt must keep its role and editing rules:\n%s", out)
	}
	if strings.Contains(out, "- Ask:") || strings.Contains(out, "askUser") {
		t.Fatalf("child prompt must not include ask instructions:\n%s", out)
	}
}

func TestPromptStaticSectionsUnderTwoKilobytes(t *testing.T) {
	static := len(identityPrompt) + len(voiceRules) + len(strings.Join(filteredMethodRules(nil), "\n")) +
		len(editingRules) + len(verificationRules) + len(outputRules) + len(baseGuidelines) + len(askGuideline)
	if static >= 2<<10 {
		t.Fatalf("static prompt text is %d bytes", static)
	}
}
