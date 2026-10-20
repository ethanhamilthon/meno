package prompt

import (
	"strings"
	"testing"
)

func TestBashDescriptionHoldsTheBashRules(t *testing.T) {
	got := BashToolDescription()
	for _, want := range []string{
		"meno mcp", "meno tasks", "meno ask", "meno view", "models: [--provider --refresh]", "meno --help",
		"Use relative paths; do not prefix commands with cd to that directory.",
		"Use cd only when a command needs a different directory.",
		"Directory changes do not persist between calls",
		"[exit N] or [cancelled]",
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
	} {
		if !strings.Contains(got, want) {
			t.Errorf("description misses %q", want)
		}
	}
	if strings.Contains(got, "environment section") || strings.Contains(got, "<skills>") || strings.Contains(got, "<editing>") {
		t.Error("description must not point at prompt sections or use XML tags")
	}
	if len(got) > 3500 {
		t.Errorf("description grew to %d bytes", len(got))
	}
}
