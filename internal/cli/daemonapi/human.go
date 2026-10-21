package daemonapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"meno/internal/cli"
	"meno/internal/client"
	"meno/internal/proto"
)

// Daemon reports the state of the daemon. Start, stop and restart are done by
// "meno daemon" itself, which runs before this API is used.
func (a *API) Daemon(action string, force bool) (cli.DaemonStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := a.c.Hello(ctx)
	if err != nil {
		return cli.DaemonStatus{}, nil
	}
	return cli.DaemonStatus{Running: true, PID: h.PID, Version: h.Version}, nil
}

// Sessions lists the sessions, optionally of one directory.
func (a *API) Sessions(cwd string) ([]cli.Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := a.c.ListSessions(ctx, proto.SessionsListRequest{Cwd: cwd})
	if err != nil {
		return nil, wrap(err)
	}
	out := make([]cli.Session, 0, len(r.Sessions))
	for _, s := range r.Sessions {
		status := "idle"
		switch {
		case s.NeedsYou:
			status = "needs you"
		case s.Running:
			status = "running"
		}
		out = append(out, cli.Session{ID: s.ID, Name: s.Name, CWD: s.Cwd, Status: status})
	}
	return out, nil
}

// Send writes a message to a session and, unless Detach is set, waits for the
// run to end and returns the final text.
func (a *API) Send(r cli.SendRequest) (cli.SendResult, error) {
	ctx := context.Background()
	id := r.SessionID
	if id == "" && !r.New {
		id = os.Getenv("MENO_SESSION")
	}
	if id == "" {
		cwd, _ := os.Getwd()
		c, err := a.c.CreateSession(ctx, proto.SessionCreateRequest{Cwd: cwd, ClientID: clientID()})
		if err != nil {
			return cli.SendResult{}, wrap(err)
		}
		id = c.Session.ID
	}
	snap, err := a.c.Snapshot(ctx, id)
	if err != nil {
		return cli.SendResult{}, wrap(err)
	}
	m, err := a.c.Message(ctx, id, proto.MessageRequest{Text: r.Text, ClientID: clientID()})
	if err != nil {
		return cli.SendResult{}, wrap(err)
	}
	res := cli.SendResult{SessionID: id, RequestID: m.RequestID, Queued: m.Queued}
	if r.Detach || m.Queued {
		return res, nil
	}
	text, err := a.follow(ctx, id, snap.Seq)
	res.Done, res.Text = err == nil, text
	return res, err
}

// follow reads the event stream until the run ends and returns what the agent said.
func (a *API) follow(ctx context.Context, id string, after uint64) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var text string
	var failure error
	finished := false
	end := func() error { finished = true; cancel(); return errStop }
	err := a.c.Stream(ctx, id, after, "", func(e proto.Event) error {
		switch e.Type {
		case proto.EventTextDelta:
			var d proto.TextDeltaData
			if decode(e.Data, &d) == nil {
				text += d.Text
			}
		case proto.EventTurnAdded:
			var d proto.TurnAddedData
			if decode(e.Data, &d) == nil && d.Turn.Role == "assistant" {
				if d.Turn.Content != "" {
					text = d.Turn.Content // the persisted final answer
				} else {
					text = "" // a tool-call turn: the next answer starts fresh
				}
			}
		case proto.EventError:
			var d proto.ErrorData
			_ = decode(e.Data, &d)
			failure = errors.New(d.Message)
			return end()
		case proto.EventAborted:
			failure = errors.New("the run was stopped")
			return end()
		case proto.EventDone:
			return end()
		}
		return nil
	})
	if failure != nil {
		return text, failure
	}
	if err != nil && !finished {
		return text, err
	}
	if !finished {
		return text, errInterrupted
	}
	return text, nil
}

var errInterrupted = errors.New("the stream closed before the run finished")
var errStop = errors.New("stop")

// Tail returns the events of a session: the whole stream of a running session
// until it ends, or the recent turns of an idle one.
func (a *API) Tail(id string) ([]cli.Event, error) {
	ctx := context.Background()
	snap, err := a.c.Snapshot(ctx, id)
	if err != nil {
		return nil, wrap(err)
	}
	var out []cli.Event
	if !snap.Running {
		for i, t := range snap.Turns {
			out = append(out, cli.Event{Sequence: uint64(i + 1), Type: "turn", Data: map[string]any{"role": t.Role, "text": turnText(t)}})
		}
		return out, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := false
	err = a.c.Stream(ctx, id, snap.Seq, "", func(e proto.Event) error {
		out = append(out, toEvent(e))
		if e.Type == proto.EventDone || e.Type == proto.EventError || e.Type == proto.EventAborted {
			finished = true
			cancel()
			return errStop
		}
		return nil
	})
	if err != nil && !finished {
		return out, err
	}
	return out, nil
}

func turnText(t proto.Turn) string {
	if t.Content != "" {
		return t.Content
	}
	for _, p := range t.Parts {
		if p.Text != "" {
			return p.Text
		}
		if p.Call != nil {
			return fmt.Sprintf("$ %s", p.Call.Command)
		}
	}
	return ""
}

var _ = client.New
