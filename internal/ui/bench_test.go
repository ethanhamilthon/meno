package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
)

func longTranscript(n int) []agent.Turn {
	var turns []agent.Turn
	for i := 0; i < n; i++ {
		turns = append(turns,
			agent.Turn{Role: llm.RoleUser, Text: fmt.Sprintf("question %d about the project layout and its tests", i)},
			agent.Turn{Role: llm.RoleAssistant, Text: "Let me look.", CallID: "c", Code: "const files = fs.glob('**/*.go');\nfiles.slice(0, 5).map(f => f.toUpperCase())", Output: strings.Repeat("some output line\n", 6), Time: time.Now()},
			agent.Turn{Role: llm.RoleAssistant, Text: "## Result\n\n- item one\n- item two with `code`\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"},
		)
	}
	return turns
}

// BenchmarkBuildLinesLongTranscript lays out a 300-turn conversation, the
// work the UI repeats when the timeline changes.
func BenchmarkBuildLinesLongTranscript(b *testing.B) {
	turns := longTranscript(100)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = BuildLines(turns, 100, JSOpts{})
	}
}

func BenchmarkHighlightAndFold(b *testing.B) {
	code := strings.Repeat("const s = `line one\nline two\nline three $"+"{x}`; run('a', \"b\"); // c\n", 20)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CodeLinesOpt(code, BgJSDone, 100, 0, CodeOpts{FoldStrings: true, FoldLines: 1})
	}
}
