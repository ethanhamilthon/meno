package prompt

import (
	"strings"
	"testing"
)

func TestBashDescriptionDescribesTheTool(t *testing.T) {
	got := BashToolDescription()
	for _, want := range []string{
		"fresh shell", "do not persist between calls",
		"meno mcp", "tools SERVER [TOOL]", "input schema", "$MENO_HOME/mcp.json", "meno mcp add --help", "meno tasks", "meno ask FILE|-", "meno view IMAGE", "meno models [--provider --refresh]",
		"meno sessions [--cwd DIR", "meno sessions show ID", "meno send (-s ID|--new|--fork SESSION)", "--model PROVIDER/MODEL", "meno tail ID...", "meno help [TOPIC]",
		"meno --help", "[exit N] or [cancelled]", "already running the message is only queued",
		"Never create new sessions or send messages to other sessions without explicit user permission.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("description misses %q", want)
		}
	}
	for _, banned := range []string{"Editing:", "Verification:", "Output:", "<skills>", "environment section"} {
		if strings.Contains(got, banned) {
			t.Errorf("policy %q belongs in the system prompt, not the tool description", banned)
		}
	}
}

func TestSystemPromptHasNoWorkingRules(t *testing.T) {
	got := Build(Input{})
	for _, banned := range []string{"working-rules", "Editing:", "Verification:", "Output:"} {
		if strings.Contains(got, banned) {
			t.Errorf("system prompt contains %q", banned)
		}
	}
}
