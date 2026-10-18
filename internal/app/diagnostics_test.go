package app

import (
	"strings"
	"testing"
)

func TestCacheReportShowsPrefixSharingWithoutText(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	say(t, a, "SECRET-first")
	say(t, a, "SECRET-second")
	a.runCommand("cache")
	n := a.lastNotice()
	if !strings.Contains(n, "all a prefix of this one") || strings.Contains(n, "SECRET") || !strings.Contains(n, "system ") {
		t.Fatalf("report: %q", n)
	}
	// A changed system prompt shares nothing.
	a.st.Reqs[len(a.st.Reqs)-1].System += " changed"
	a.runCommand("cache")
	if n := a.lastNotice(); !strings.Contains(n, "nothing can be served") {
		t.Fatalf("report: %q", n)
	}
}
