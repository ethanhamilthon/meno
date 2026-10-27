package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
)

// A session created with NoAsk answers `meno ask` with the forbidden exit code
// and a message that sends the model to the chat. No form is ever opened.
func TestAskIsRefusedInANoAskSession(t *testing.T) {
	home, eng, _, _, rpc := startRig(t, &scripted{})
	made, err := eng.CreateSession(context.Background(), api.SessionCreateRequest{Cwd: t.TempDir(), ClientID: "noask", NoAsk: true})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Snapshot(context.Background(), made.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	dapi := New(home, rpc, snap.Session.ID, snap.AgentToken, "call2")
	var out, errb strings.Builder
	code := Run([]string{"ask", "-"}, IO{In: strings.NewReader(`{"questions":[{"id":"q","text":"?","type":"text"}]}`), Out: &out, Err: &errb, Dir: t.TempDir()}, dapi)
	if code != ExitForbidden {
		t.Fatalf("exit %d, want %d (%s %s)", code, ExitForbidden, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "write your question directly in the chat") {
		t.Fatalf("stderr %q", errb.String())
	}
	time.Sleep(20 * time.Millisecond)
	after, err := eng.Snapshot(context.Background(), snap.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Ask != nil {
		t.Fatalf("a form was opened: %+v", after.Ask)
	}
}
