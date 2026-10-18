package agent

import (
	"fmt"
	"strings"
	"testing"

	"meno/internal/llm"
)

func benchSession(n int) *Session {
	s := NewSession("/w", "m")
	for i := 0; i < n; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: fmt.Sprintf("message %d", i)})
		s.Append(Turn{Role: llm.RoleAssistant, Text: strings.Repeat("answer ", 40), CallID: "c", Code: "1+1", Output: "2"})
	}
	return s
}

// BenchmarkToMsgs builds the provider request history of a long session; it
// runs before every model request.
func BenchmarkToMsgs(b *testing.B) {
	s := benchSession(300)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.ToMsgs()
	}
}

func BenchmarkSaveSession(b *testing.B) {
	home := b.TempDir()
	defer CloseSessionStores()
	s := benchSession(300)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := SaveSession(home, s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFingerprint(b *testing.B) {
	msgs := benchSession(300).ToMsgs()
	req := llm.Request{Model: "m", System: strings.Repeat("s", 8000), Msgs: msgs, Tools: true}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = llm.FingerprintOf(req)
	}
}
