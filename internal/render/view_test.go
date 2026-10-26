package render

import (
	"strings"
	"testing"
	"time"
)

func viewTurn() Turn {
	return Turn{Role: RoleAssistant, Title: "List files", Code: "ls -la\necho 2\necho 3\necho 4\necho 5\necho 6\necho 7",
		Output: "o1\no2\no3\no4\no5\no6\no7", Thinking: "pondering", ElapsedMs: 1200}
}

func TestViewModesCutWhatTheyShould(t *testing.T) {
	turns := []Turn{viewTurn(), {Role: RoleAssistant, Text: "the answer"}}
	full := summaryText(BuildLines(turns, 80, JSOpts{View: ViewFull}))
	for _, want := range []string{"pondering", "List files", "echo 7", "o5", "the answer"} {
		if !strings.Contains(full, want) {
			t.Errorf("full lacks %q:\n%s", want, full)
		}
	}
	if strings.Contains(full, "o6") {
		t.Errorf("full shows more than five output rows:\n%s", full)
	}
	bash := summaryText(BuildLines(turns, 80, JSOpts{View: ViewBashOnly}))
	if !strings.Contains(bash, "ls -la") || !strings.Contains(bash, "echo 5") || strings.Contains(bash, "echo 6") {
		t.Errorf("bash only must cut the command to five rows:\n%s", bash)
	}
	for _, hidden := range []string{"pondering", "o1"} {
		if strings.Contains(bash, hidden) {
			t.Errorf("bash only shows %q:\n%s", hidden, bash)
		}
	}
	compact := summaryText(BuildLines(turns, 80, JSOpts{View: ViewCompact}))
	if strings.TrimSpace(compact) != "◆ the answer" && !strings.Contains(compact, "the answer") {
		t.Fatalf("compact lost the answer:\n%s", compact)
	}
	for _, hidden := range []string{"pondering", "o1", "ls -la", "List files"} {
		if strings.Contains(compact, hidden) {
			t.Errorf("compact shows %q:\n%s", hidden, compact)
		}
	}
}

func TestCompactLiveRow(t *testing.T) {
	line := CompactLive("sleep 5 &&\n   echo done", 83*time.Second, 60)
	text := line.Text()
	if !strings.Contains(text, "sleep 5 && echo done") || !strings.HasSuffix(strings.TrimSpace(text), "1m 23s") {
		t.Fatalf("row: %q", text)
	}
	if TextWidth(text) > 60 {
		t.Fatalf("row is wider than the screen: %d", TextWidth(text))
	}
	if narrow := CompactLive(strings.Repeat("x", 200), time.Second, 30); TextWidth(narrow.Text()) > 30 {
		t.Fatal("long command overflows")
	}
}
