package app

import "testing"

// A wholly unset session previews the defaults as the daemon will resolve
// them; a partial explicit pair must not be masked by them.
func TestSelectionPreviewFollowsTheDaemonRules(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Settings.Provider, a.st.Settings.Model, a.st.Settings.Effort = "dp", "dm", "low"
	for _, c := range []struct{ prov, model, wantP, wantM string }{
		{"", "", "dp", "dm"},
		{"p", "m", "p", "m"},
		{"p", "", "p", ""},
		{"", "m", "", "m"},
	} {
		a.st.Session.Provider, a.st.Session.Model = c.prov, c.model
		p, m, _ := a.selection()
		if p != c.wantP || m != c.wantM {
			t.Errorf("session %q/%q: selection %q/%q, want %q/%q", c.prov, c.model, p, m, c.wantP, c.wantM)
		}
	}
}
