package cli

import (
	"strings"
	"testing"
)

// The help pages are read by models: they stay short, and the pages for mcp
// keep the facts a caller cannot guess.
func TestHelpPagesStayShort(t *testing.T) {
	for _, topic := range Topics() {
		text, err := Text(topic)
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(text, "\n"); n > 60 {
			t.Errorf("%s help has %d lines, the limit is 60", topic, n)
		}
	}
}

func TestMCPHelpStatesWhatACallerCannotGuess(t *testing.T) {
	for topic, phrases := range map[string][]string{
		"mcp":         {"meno mcp tools SERVER TOOL", "$MENO_HOME/mcp.json"},
		"mcp-servers": {"running", "starting", "stopped", "disabled", "error: MESSAGE"},
		"mcp-tools":   {"input schema", "--json", "inputSchema"},
		"mcp-call":    {"structuredContent", "isError", "resultId", "exits 0", "artifacts"},
		"mcp-results": {"10000 bytes", "65536", "nextOffset"},
		"mcp-add":     {"$MENO_HOME/mcp.json", "mcp.json.bak", "jq .", "meno mcp restart NAME", "never print it whole", "cannot create a server"},
		"mcp-restart": {"returns at once", "meno mcp servers"},
		"meno":        {"4 doctor found warnings", "5 a session waits", "124"},
	} {
		text, err := Text(topic)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range phrases {
			if !strings.Contains(text, p) {
				t.Errorf("%s help does not say %q", topic, p)
			}
		}
	}
}
