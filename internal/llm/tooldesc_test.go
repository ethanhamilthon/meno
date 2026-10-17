package llm

import (
	"strings"
	"testing"
)

func TestToolDescriptionNamesNoGlobals(t *testing.T) {
	for _, bad := range []string{"fs.", "sh(", "fetch", "mcp", "askUser", "console"} {
		if strings.Contains(ToolDescription, bad) {
			t.Fatalf("tool description mentions %q: %s", bad, ToolDescription)
		}
	}
	if n := strings.Count(ToolDescription, ". ") + 1; n != 2 {
		t.Fatalf("want two sentences, got %d", n)
	}
}
