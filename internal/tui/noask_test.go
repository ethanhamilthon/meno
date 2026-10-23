package tui

import (
	"context"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
)

// A window started with --no-ask creates its first session and every /new
// session with the ask tool turned off; a normal window never does.
func TestNoAskWindowCreatesNoAskSessions(t *testing.T) {
	for _, noAsk := range []bool{false, true} {
		scr := tcell.NewSimulationScreen("UTF-8")
		if err := scr.Init(); err != nil {
			t.Fatal(err)
		}
		scr.SetSize(96, 30)
		paths := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
		r := startRig(t, paths.Home, &scriptedClient{})
		var opts []Option
		if noAsk {
			opts = append(opts, WithNoAsk())
		}
		a, err := New(scr, paths, r.cl, opts...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(a.Close)
		check := func(when string) {
			snap, err := r.eng.Snapshot(context.Background(), a.st.Runtime.ID())
			if err != nil {
				t.Fatal(err)
			}
			if snap.Session.NoAsk != noAsk {
				t.Errorf("noAsk=%v %s: session NoAsk=%v", noAsk, when, snap.Session.NoAsk)
			}
		}
		check("first session")
		a.newSession()
		check("/new session")
	}
}
