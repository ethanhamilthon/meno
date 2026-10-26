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

func TestOperatingRulesAppearOnceInSystemPrompt(t *testing.T) {
	got := Build(Input{})
	for _, want := range []string{
		"Editing:", "Verification:", "Output:",
		"Read a file before you change it. Change part of a file with sed or a quoted heredoc, not by sending the whole file again. Do not overwrite content you have not read.",
		"Find with rg when it exists (check with command -v rg), otherwise grep -rn. Never guess a path or a symbol.",
		"Check the smallest thing that can fail: the package you touched, not the whole repo.",
		"Run the full suite once, before you say the work is done.",
		"A failing test is not done until it passes or until you say why it cannot.",
		"Do not run the same command twice unchanged.",
		"Ask the program for less: head, tail, a filter, a narrower flag. Do not dump a directory or a whole log.",
		"Return the value you need, not the whole object.",
		"Everything a call prints stays in the context for the rest of the run and is paid for again in every request.",
		"Use relative paths; do not prefix commands with cd",
	} {
		if n := strings.Count(got, want); n != 1 {
			t.Errorf("%q appears %d times, want 1", want, n)
		}
	}
}
