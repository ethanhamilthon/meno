package cli

import (
	"context"
	"strings"
	"time"

	"meno/internal/api"
)

func sessionStatus(running, needsYou bool) string {
	switch {
	case needsYou:
		return "needs you"
	case running:
		return "running"
	}
	return "idle"
}

func toSession(s api.SessionItem) Session {
	return Session{
		ID: s.ID, Name: s.Name, CWD: s.Cwd, Status: sessionStatus(s.Running, s.NeedsYou),
		Provider: s.Provider, Model: s.Model, Effort: s.Effort,
		SpawnedBy: s.SpawnedBy, ForkOf: s.ForkOf, System: s.System, NoAsk: s.NoAsk,
		Turns: s.TurnCount, UpdatedAt: s.UpdatedAt,
	}
}

// statusMatches compares a status filter with a session status; "needs-you",
// "needs_you" and "waiting" all name "needs you".
func statusMatches(want, have string) bool {
	norm := func(v string) string {
		v = strings.ToLower(strings.NewReplacer("-", " ", "_", " ").Replace(v))
		if v == "waiting" {
			return "needs you"
		}
		return v
	}
	return norm(want) == norm(have)
}

// Sessions lists the sessions that match the filters, newest first.
func (a *DaemonAPI) Sessions(r SessionsRequest) ([]Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	list, err := a.c.ListSessions(ctx, api.SessionsListRequest{Cwd: r.CWD})
	if err != nil {
		return nil, wrap(err)
	}
	parent := r.SpawnedBy
	if parent == "me" {
		parent = a.session
	}
	out := make([]Session, 0, len(list.Sessions))
	for _, it := range list.Sessions {
		s := toSession(it)
		switch {
		case r.Status != "" && !statusMatches(r.Status, s.Status),
			parent != "" && s.SpawnedBy != parent,
			r.TitlePrefix != "" && !strings.HasPrefix(s.Name, r.TitlePrefix):
			continue
		}
		out = append(out, s)
		if r.Limit > 0 && len(out) >= r.Limit {
			break
		}
	}
	return out, nil
}

func toUsage(u *api.UsageData) *Usage {
	if u == nil {
		return nil
	}
	return &Usage{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, CachedTokens: u.Cached, Cost: u.Cost}
}

func toQuestion(f *api.AskForm) *PendingQuestion {
	if f == nil {
		return nil
	}
	q := &PendingQuestion{ID: f.AskID, Title: f.Title}
	for _, x := range f.Questions {
		q.Questions = append(q.Questions, QuestionItem{ID: x.ID, Prompt: x.Prompt, Type: x.Type, Options: x.Options})
	}
	return q
}

// lastAnswer is the text of the last assistant turn, or what the running turn has said so far.
func lastAnswer(snap api.Snapshot) string {
	if snap.Running && snap.Live.Text != "" {
		return snap.Live.Text
	}
	for i := len(snap.Turns) - 1; i >= 0; i-- {
		if t := snap.Turns[i]; t.Role == "assistant" && t.Content != "" {
			return t.Content
		}
	}
	return ""
}

// SessionShow reports one session: state, open question and the latest answer.
func (a *DaemonAPI) SessionShow(id string) (SessionDetail, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	snap, err := a.c.Snapshot(ctx, id)
	if err != nil {
		return SessionDetail{}, wrap(err)
	}
	it := snap.Session
	return SessionDetail{
		Session:   toSession(it),
		Answer:    lastAnswer(snap),
		LastError: snap.LastError,
		Question:  toQuestion(snap.Ask),
		Queued:    snap.Queued,
		Tasks:     snap.TaskCount,
		Usage:     toUsage(snap.TotalUsage),
	}, nil
}

// SessionCancel stops the run of a session. A session with nothing to stop is an error.
func (a *DaemonAPI) SessionCancel(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	snap, err := a.c.Snapshot(ctx, id)
	if err != nil {
		return wrap(err)
	}
	if !snap.Running && snap.Ask == nil {
		return &APIError{Code: "conflict", Message: "session " + id + " has no run to stop"}
	}
	return wrap(a.c.Stop(ctx, id, api.StopRequest{}))
}

// SessionAnswer answers or cancels the open question of a session.
func (a *DaemonAPI) SessionAnswer(r AnswerRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return wrap(a.c.AnswerAsk(ctx, r.SessionID, r.QuestionID, api.AskAnswerRequest{Answers: r.Answers, Cancel: r.Cancel}))
}
