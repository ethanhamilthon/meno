package tui

import "testing"

func TestOfflineKeepsDraftAndShowsStatus(t *testing.T) {
	a, _ := newTestApp(t)
	a.setInput([]rune("do the thing"))
	a.setConnLost(a.st.Runtime.ID(), true)
	if a.connLabel() == "" {
		t.Fatal("offline must be visible")
	}
	a.submit()
	if string(a.st.Input) != "do the thing" {
		t.Fatalf("draft lost: %q", string(a.st.Input))
	}
	a.setConnLost(a.st.Runtime.ID(), false)
	if a.connLabel() != "" {
		t.Fatal("a fresh snapshot clears the offline state")
	}
}
