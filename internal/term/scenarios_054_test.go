//go:build termtest

package term

import (
	"strings"
	"testing"
	"time"
)

func TestCompactFooterAndExplicitEdit(t *testing.T) {
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
	if !strings.Contains(screen[len(screen)-2], "alpha beta gamma delta") {
		t.Fatalf("extra composer padding: %q", screen[len(screen)-2])
	}
	s.Cmd("edit")
	s.Send("0x")
	s.Send(Esc)
	s.WaitFor("lpha beta gamma delta")
}
