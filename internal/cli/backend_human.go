package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"meno/internal/api"
	"meno/internal/client"
)

// Daemon reports the state of the daemon. Start, stop and restart are done by
// "meno daemon" itself, which runs before this DaemonAPI is used.
func (a *DaemonAPI) Daemon(action string, force bool) (DaemonStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := a.c.Hello(ctx)
	if err != nil {
		return DaemonStatus{}, nil
	}
	return DaemonStatus{Running: true, PID: h.PID, Version: h.Version}, nil
}

// Sessions lists the sessions, optionally of one directory.
func (a *DaemonAPI) Sessions(cwd string) ([]Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := a.c.ListSessions(ctx, api.SessionsListRequest{Cwd: cwd})
	if err != nil {
		return nil, wrap(err)
	}
	out := make([]Session, 0, len(r.Sessions))
	for _, s := range r.Sessions {
		status := "idle"
		switch {
		case s.NeedsYou:
			status = "needs you"
		case s.Running:
			status = "running"
		}
		out = append(out, Session{ID: s.ID, Name: s.Name, CWD: s.Cwd, Status: status})
	}
	return out, nil
}

// Send writes a message to a session and, unless Detach is set, waits for the
// run to end and returns the final text.
func (a *DaemonAPI) Send(r SendRequest) (SendResult, error) {
	ctx := context.Background()
	id := r.SessionID
	if id == "" && !r.New {
		id = os.Getenv("MENO_SESSION")
	}
	if id == "" {
		cwd, _ := os.Getwd()
		c, err := a.c.CreateSession(ctx, api.SessionCreateRequest{Cwd: cwd, ClientID: clientID()})
		if err != nil {
			return SendResult{}, wrap(err)
		}
		id = c.Session.ID
	}
	snap, err := a.c.Snapshot(ctx, id)
	if err != nil {
		return SendResult{}, wrap(err)
	}
	m, err := a.c.Message(ctx, id, api.MessageRequest{Text: r.Text, ClientID: clientID()})
	if err != nil {
		return SendResult{}, wrap(err)
	}
	res := SendResult{SessionID: id, RequestID: m.RequestID, Queued: m.Queued}
	if r.Detach || m.Queued {
		return res, nil
	}
	text, err := a.follow(ctx, id, snap.Seq)
	res.Done, res.Text = err == nil, text
	return res, err
}

// follow reads the event stream until the run ends and returns what the agent said.
func (a *DaemonAPI) follow(ctx context.Context, id string, after uint64) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var text string
	var failure error
	finished := false
	end := func() error { finished = true; cancel(); return errStop }
	err := a.c.Stream(ctx, id, after, "", func(e api.Event) error {
		switch e.Type {
		case api.EventTextDelta:
			var d api.TextDeltaData
			if decode(e.Data, &d) == nil {
				text += d.Text
			}
		case api.EventTurnAdded:
			var d api.TurnAddedData
			if decode(e.Data, &d) == nil && d.Turn.Role == "assistant" {
				if d.Turn.Content != "" {
					text = d.Turn.Content // the persisted final answer
				} else {
					text = "" // a tool-call turn: the next answer starts fresh
				}
			}
		case api.EventError:
			var d api.ErrorData
			_ = decode(e.Data, &d)
			failure = errors.New(d.Message)
			return end()
		case api.EventAborted:
			failure = errors.New("the run was stopped")
			return end()
		case api.EventDone:
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
func (a *DaemonAPI) Tail(id string) ([]Event, error) {
	ctx := context.Background()
	snap, err := a.c.Snapshot(ctx, id)
	if err != nil {
		return nil, wrap(err)
	}
	var out []Event
	if !snap.Running {
		for i, t := range snap.Turns {
			out = append(out, Event{Sequence: uint64(i + 1), Type: "turn", Data: map[string]any{"role": t.Role, "text": turnText(t)}})
		}
		return out, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := false
	err = a.c.Stream(ctx, id, snap.Seq, "", func(e api.Event) error {
		out = append(out, toEvent(e))
		if e.Type == api.EventDone || e.Type == api.EventError || e.Type == api.EventAborted {
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

func turnText(t api.Turn) string {
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
