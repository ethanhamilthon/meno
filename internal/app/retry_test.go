package app

import (
	"strings"
	"testing"
	"time"

	"meno/internal/agent"
)

func TestRetryShowsCountdownAndDropsPartialOutput(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Running = true
	a.onAgent(agent.Ev{Kind: agent.EvText, Text: "half an ans"})
	a.onAgent(agent.Ev{Kind: agent.EvRetry, Attempt: 2, Max: 5, Delay: 4 * time.Second, Err: "http 503: service unavailable"})
	if a.st.LiveText != "" {
		t.Fatal("partial text of the failed request must be dropped")
	}
	a.draw()
	text := screenText(scr)
	if !strings.Contains(text, "retry 2/5 in 4s") || !strings.Contains(text, "503") {
		t.Fatalf("status misses the retry:\n%s", text)
	}
	if !strings.Contains(a.lastNotice(), "retry 2/5") {
		t.Fatalf("no timeline line: %q", a.lastNotice())
	}
	a.st.Retry.Until = time.Now().Add(-time.Second)
	if !strings.Contains(a.retryText(), "sending") {
		t.Fatal(a.retryText())
	}
	a.onAgent(agent.Ev{Kind: agent.EvText, Text: "ok"})
	if a.st.Retry != nil || a.retryText() != "" {
		t.Fatal("retry status must clear once the stream resumes")
	}
}
