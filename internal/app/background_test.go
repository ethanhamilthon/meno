package app

import (
	"strings"
	"testing"
)

func TestStatusBarShowsBackgroundWorkNextToTheModel(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.draw()
	if strings.Contains(screenText(scr), "◆") {
		t.Fatal("marker shown without background work")
	}
	a.onBgLoaded(bgLoaded{map[string]int{a.st.Session.ID: 2}})
	a.draw()
	if !strings.Contains(screenText(scr), "◆ 2") {
		t.Fatalf("no background marker in the status bar:\n%s", screenText(scr))
	}
	if !a.animating() {
		t.Fatal("background work must keep the marker animating")
	}
	a.onBgLoaded(bgLoaded{map[string]int{}})
	if a.animating() || a.bgInflight {
		t.Fatal("animation or inflight flag stuck after the work ended")
	}
}

func TestSessionPickerShowsBackgroundWorkPerSession(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.onBgLoaded(bgLoaded{map[string]int{a.st.Session.ID: 3}})
	a.openSessions()
	a.draw()
	if !strings.Contains(screenText(scr), "◆ 3") {
		t.Fatalf("no background marker in the session picker:\n%s", screenText(scr))
	}
}

func TestBadgeIsEmptyWithoutWork(t *testing.T) {
	if bgBadge(0) != "" || bgBadge(-1) != "" || bgBadge(4) != "◆ 4" {
		t.Fatal("badge text")
	}
}
