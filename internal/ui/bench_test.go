package ui

import (
	"fmt"
	"strings"
	"testing"
)

func longTranscript(n int) []Turn {
	var turns []Turn
	for i := 0; i < n; i++ {
		turns = append(turns,
			Turn{Role: RoleUser, Text: fmt.Sprintf("question %d about the project layout and its tests", i)},
			Turn{Role: RoleAssistant, Text: "Let me look.", CallID: "c", Code: "const files = fs.glob('**/*.go');\nfiles.slice(0, 5).map(f => f.toUpperCase())", Output: strings.Repeat("some output line\n", 6)},
			Turn{Role: RoleAssistant, Text: "## Result\n\n- item one\n- item two with `code`\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"},
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
