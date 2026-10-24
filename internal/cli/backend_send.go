package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"meno/internal/api"
)

var (
	errInterrupted = errors.New("the stream closed before the run finished")
	errStop        = errors.New("stop")
	errAborted     = errors.New("the run was stopped")
)

// isAborted reports whether the run Send waited for was stopped.
func isAborted(err error) bool { return errors.Is(err, errAborted) }

// Send writes a message to a session and, unless Detach is set, waits for the
// run to end and returns the final text. The session is an existing one
// (SessionID), a new one (New) or a fork of another (Fork). A run that Send
// started in a session it created is cancelled when Send is interrupted.
func (a *DaemonAPI) Send(r SendRequest) (SendResult, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var res SendResult
	id, created := r.SessionID, false
	switch {
	case r.Fork != "":
		f, err := a.c.Fork(ctx, r.Fork, api.ForkRequest{
			Turn: r.At, ClientID: clientID(), Model: r.Model, Effort: r.Effort, Title: r.Title, Cwd: r.CWD,
			System: r.System, NoAsk: r.NoAsk, SpawnedBy: a.session,
		})
		if err != nil {
			return res, wrap(err)
		}
		id, created = f.Session.ID, true
		res.ForkOf, res.Warnings = r.Fork, f.Warnings
	case r.New:
		cwd := r.CWD
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		c, err := a.c.CreateSession(ctx, api.SessionCreateRequest{
			Cwd: cwd, ClientID: clientID(), NoAsk: r.NoAsk, Model: r.Model, Effort: r.Effort,
			Title: r.Title, System: r.System, SpawnedBy: a.session,
		})
		if err != nil {
			return res, wrap(err)
		}
		id, created = c.Session.ID, true
	}
	res.SessionID = id
	snap, err := a.c.Snapshot(ctx, id)
	if err != nil {
		return a.discard(res, id, created, wrap(err))
	}
	if !created && (r.Model != "" || r.Effort != "") {
		warning, err := a.changeModel(ctx, id, r)
		if err != nil {
			return res, err
		}
		res.Warnings = append(res.Warnings, warning)
	}
	m, err := a.c.Message(ctx, id, api.MessageRequest{Text: r.Text, ClientID: clientID(), Client: "cli"})
	if err != nil {
		return a.discard(res, id, created, wrap(err))
	}
	res.RequestID, res.Queued = m.RequestID, m.Queued
	switch {
	case m.Queued:
		res.Status = "queued"
		return res, nil
	case r.Detach:
		res.Status = "detached"
		return res, nil
	}
	return a.wait(ctx, res, r, created, snap.Seq)
}

// discard deletes a session that was created for a send that then failed, so
// it does not count against the worker limit, and returns err.
func (a *DaemonAPI) discard(res SendResult, id string, created bool, err error) (SendResult, error) {
	if created {
		_ = a.c.DeleteSession(context.Background(), id)
	}
	return res, err
}

// wait follows the run and fills the result.
func (a *DaemonAPI) wait(ctx context.Context, res SendResult, r SendRequest, created bool, after uint64) (SendResult, error) {
	fctx := ctx
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		fctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	text, ask, err := a.follow(fctx, res.SessionID, after)
	res.Text = text
	switch {
	case ask != nil:
		res.Status, res.Question = "waiting", toQuestion(ask)
		return res, nil
	case err == nil:
		res.Status, res.Done = "done", true
		if snap, e := a.c.Snapshot(context.Background(), res.SessionID); e == nil {
			res.Usage = toUsage(snap.TotalUsage)
		}
		return res, nil
	case isAborted(err):
		res.Status, res.Error = "aborted", err.Error()
	case errors.Is(fctx.Err(), context.DeadlineExceeded) && ctx.Err() == nil:
		res.Status = "timeout"
		return res, &APIError{Code: "wait_timeout", Message: fmt.Sprintf("session %s did not finish within %s; its run goes on", res.SessionID, r.Timeout)}
	case ctx.Err() != nil:
		res.Status, res.Error = "interrupted", "interrupted"
		if r.CancelOnExit || created && !r.KeepRunning {
			if a.c.Stop(context.Background(), res.SessionID, api.StopRequest{}) == nil {
				res.Status = "cancelled"
			}
		}
		fate := "goes on"
		if res.Status == "cancelled" {
			fate = "was cancelled"
		}
		err = fmt.Errorf("interrupted; the run of session %s %s", res.SessionID, fate)
	default:
		res.Status, res.Error = "failed", err.Error()
	}
	return res, err
}

// changeModel switches the model or effort of an existing session; the daemon
// checks the choice and refuses while the session is running.
func (a *DaemonAPI) changeModel(ctx context.Context, id string, r SendRequest) (string, error) {
	_, err := a.c.PatchSession(ctx, id, api.PatchSessionRequest{Select: &api.ModelSelection{Model: r.Model, Effort: r.Effort}})
	if err != nil {
		return "", wrap(err)
	}
	return "the model of session " + id + " changed; its prompt cache restarts", nil
}

// follow reads the event stream until the run ends and returns what the agent
// said. It also stops, without an error, when the agent opens a question.
func (a *DaemonAPI) follow(ctx context.Context, id string, after uint64) (string, *api.AskForm, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var text string
	var failure error
	var ask *api.AskForm
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
		case api.EventAskOpen:
			var f api.AskForm
			if decode(e.Data, &f) == nil && f.AskID != "" {
				ask = &f
				return end()
			}
		case api.EventError:
			var d api.ErrorData
			_ = decode(e.Data, &d)
			failure = errors.New(d.Message)
			return end()
		case api.EventAborted:
			failure = errAborted
			return end()
		case api.EventDone:
			return end()
		}
		return nil
	})
	if ask != nil {
		return text, ask, nil
	}
	if failure != nil {
		return text, nil, failure
	}
	if err != nil && !finished {
		return text, nil, err
	}
	if !finished {
		return text, nil, errInterrupted
	}
	return text, nil, nil
}
