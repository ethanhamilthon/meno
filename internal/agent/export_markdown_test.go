package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/llm"
)

func TestExportMarkdownConfigurationsCacheAndSafeFences(t *testing.T) {
	s := ExportSnapshot{Model: "current", Configurations: []RequestConfig{{ID: "abc", System: "prompt\n```\ninside", Tool: llm.Request{Tools: true, ToolName: "bash"}.ExportToolDefinition()}}, Requests: []RequestRecord{
		{Step: "s", Attempt: 1, Config: "abc", Model: "old", Outcome: "failed"},
		{Step: "s", Attempt: 2, Config: "abc", Model: "old", Outcome: "completed", Usage: &llm.Usage{In: 12, Out: 2}},
		{Step: "next", Attempt: 1, Config: "abc", Model: "new", Outcome: "completed", Usage: &llm.Usage{CacheReported: true}},
	}, Turns: []Turn{{HID: 1, Role: "user", Text: "hello", System: "system event"}, {HID: 2, Role: "assistant", Text: "explanation", Code: "echo '```'", Output: "```\nresult", CallID: "c1"}, {HID: 3, Role: "assistant", Code: "pwd", Output: "/work", CallID: "c2"}}}
	text := s.Markdown()
	path := filepath.Join("testdata", "export.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if text != string(golden) {
		t.Fatal("export Markdown differs from testdata/export.golden")
	}
	for _, want := range []string{"Current session settings", "### Configuration abc", "[abc](#configuration-abc)", "````text\nprompt\n```\ninside\n````", "| 12 | 2 | unavailable | unavailable |", "| 0 | 0 | 0 | 0 |", "Tool call: `c1`", "Tool call: `c2`", "explanation", "system event", "before history ID"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in export", want)
		}
	}
	if strings.Count(text, "### Configuration abc") != 1 {
		t.Fatal("repeated configuration")
	}
}
