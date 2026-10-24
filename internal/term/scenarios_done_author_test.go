//go:build termtest

package term

import (
	"context"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/cli"
	"meno/internal/client"
)

func TestCLISendInTUISessionDoesNotProduceDone(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) { time.Sleep(700 * time.Millisecond); q.Text("author-completion-result") }
	s := start(t, options{fake: f})
	// Persist and identify the TUI session, then cover its result to retain unread.
	s.Say("first human message")
	s.WaitFor("author-completion-result")
	s.Cmd("help")
	s.WaitFor("keybindings")
	c := client.New(s.home)
	list, err := c.ListSessions(context.Background(), api.SessionsListRequest{})
	if err != nil || len(list.Sessions) != 1 {
		t.Fatalf("sessions: %+v %v", list, err)
	}
	id := list.Sessions[0].ID
	d := cli.New(s.home, c.Socket, "", "", "")
	if _, err := d.Send(cli.SendRequest{SessionID: id, Text: "external CLI message"}); err != nil {
		t.Fatal(err)
	}
	// Leave help open, so any accidental completion remains unread.
	time.Sleep(200 * time.Millisecond)
	s.WaitGone("1 done")
	snap, err := c.Snapshot(context.Background(), id)
	if err != nil || snap.Session.Completed != 1 {
		t.Fatalf("CLI completion counted: %+v %v", snap.Session, err)
	}
	s.Send(Esc)
	s.WaitGone("keybindings")
	s.Say("another human message")
	s.WaitFor("another human message")
	s.Cmd("help")
	s.WaitFor("keybindings")
	s.WaitFor("1 done")
}
