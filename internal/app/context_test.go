package app

import (
	"strings"
	"testing"

	"meno/internal/llm"
)

func TestContextRowsIdleAndRunning(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Usage = llm.Usage{In: 1200, Out: 300, Cached: 600, CacheReported: true}
	a.st.LastUsage = llm.Usage{In: 100, Out: 10}
	text := strings.Join(a.contextRows(), "\n")
	if strings.Contains(text, "current request") || !strings.Contains(text, "session          in 1.2k · out 300 · cache hit 50%") ||
		!strings.Contains(text, "last turn        in 100 · out 10 · cache hit ?") {
		t.Fatal(text)
	}
	a.st.Running = true
	a.st.RunUsage0 = llm.Usage{In: 1000, Out: 250, Cached: 600, CacheReported: true}
	text = strings.Join(a.contextRows(), "\n")
	if !strings.Contains(text, "current request  in 200 · out 50 · cache hit 0%") {
		t.Fatal(text)
	}
}
