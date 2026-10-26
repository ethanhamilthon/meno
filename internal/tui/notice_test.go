package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/version"
)

func TestNoticesGoToTimelineNotInput(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.draw()
	a.runCommand("nope")
	a.draw()
	if out := screenText(s); !strings.Contains(out, "unknown command :nope") || !strings.Contains(out, version.Label()) {
		t.Fatalf("notice must sit over the welcome screen:\n%s", out)
	}
	if strings.Contains(visualRow(s, 27), "unknown command") {
		t.Fatal("notice leaked into the composer")
	}
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "hi"})
	a.notifyErr("first")
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "answer"})
	a.notify("second")
	out := transcriptText(a)
	i, j, k := strings.Index(out, "hi"), strings.Index(out, "first"), strings.Index(out, "answer")
	if !(i < j && j < k && k < strings.Index(out, "second")) {
		t.Fatalf("order:\n%s", out)
	}
}

// A message that was not sent stays on screen past the usual lifetime of a
// notice, until the user dismisses it; ordinary notices still fade.
func TestFailureNoticeStaysUntilDismissed(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	now := time.Now()
	noticeNow = func() time.Time { return now }
	defer func() { noticeNow = time.Now }()
	a.notifyFailure("not sent: boom")
	a.notify("fades")
	now = now.Add(time.Minute)
	a.pruneNotices()
	if got := a.currentNotices(); len(got) != 1 || !got[0].Sticky {
		t.Fatalf("notices after a minute: %+v", got)
	}
	a.onKey(keyK(tcell.KeyEsc))
	if len(a.currentNotices()) != 0 {
		t.Fatal("esc did not dismiss the failure")
	}
}
