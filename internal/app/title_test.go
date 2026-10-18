package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPlaceholderShowsTheLatestTitle(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	a.st.Running, a.st.RunStart = true, time.Now()
	cases := []struct{ phase, title, want string }{
		{"thinking", "", "thinking..."},
		{"thinking", "Read the config", "Read the config..."},
		{"running", "Run the tests", "Run the tests..."},
		{"writing", "Run the tests", "writing..."},
	}
	for _, c := range cases {
		a.st.Phase, a.st.Title = c.phase, c.title
		if got := a.placeholder(); !strings.HasPrefix(got, c.want+" ") {
			t.Errorf("phase %q title %q: %q", c.phase, c.title, got)
		}
	}
}

func TestUpdateCheckPostsANotice(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	a.SetUpdateCheck(func(context.Context) string { return "v9.9.9" })
	a.startUpdateCheck()
	pump(t, a, func() bool { return a.st.UpdateTag == "v9.9.9" })
	if segs := a.versionSegs(); len(segs) != 2 || !strings.Contains(segs[1].text, "meno update") {
		t.Fatalf("%+v", segs)
	}
}
