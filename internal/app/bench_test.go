package app

import (
	"fmt"
	"testing"

	"meno/internal/agent"
	"meno/internal/llm"
)

// BenchmarkTimelineFrame is the layout cost of one frame of a 600-turn
// conversation: the history is unchanged, only the width and options matter.
func BenchmarkTimelineFrame(b *testing.B) {
	a, scr, _ := e2eApp(b, &recClient{})
	defer scr.Fini()
	for i := 0; i < 200; i++ {
		a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: fmt.Sprintf("question %d", i)})
		a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "Let me look.", CallID: "c", Code: "fs.glob('**/*.go')", Output: "a.go\nb.go\nc.go\n"})
		a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "## Result\n\n- one\n- two\n"})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.transcriptWithSelection(100)
	}
}
