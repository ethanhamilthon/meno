//go:build termtest

package term

import (
	"strings"
	"testing"
	"time"
)

func TestPaddedFooterAndEscape(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { q.Text("done") }
	s := start(t, options{fake: f, cols: 80, rows: 24})
	s.Type("alpha beta gamma")
	s.Send(Esc)
	time.Sleep(150 * time.Millisecond)
	s.Type(" delta")
	s.WaitFor("alpha beta gamma delta")
	screen := strings.Split(strings.TrimSuffix(s.Screen(), "\n"), "\n")
	last := screen[len(screen)-1]
	if !strings.Contains(last, "fake-model |") || strings.Contains(last, "INPUT") {
		t.Fatalf("footer %q", last)
	}
	if !strings.Contains(screen[len(screen)-3], "alpha beta gamma delta") {
		t.Fatalf("input missing: %q", screen[len(screen)-2])
	}
}
