package routing

import "context"

// Context contains only the current request signals used to choose effort.
type Context struct {
	Text        string
	Modes       []string
	Attachments int
	Files       int
}

// Request asks for an effort level, or an empty level to keep Current.
type Request struct {
	Context Context
	Current string
	Allowed []string
}

// Verdict is the router's requested level and its short user-facing reason.
type Verdict struct {
	Level string
	Why   string
}

// Router selects effort for one request.
type Router interface {
	Route(ctx context.Context, r Request) (Verdict, error)
}

const hysteresisStreak = 2

// Sticky holds an applied level and an in-memory pending verdict streak.
type Sticky struct {
	Current string
	pending string
	streak  int
}

// Decide applies a verdict only after the same different level is requested twice.
func (s *Sticky) Decide(v Verdict) (level, why string) {
	if v.Level == "" {
		s.pending, s.streak = "", 0
		return s.Current, ""
	}
	if v.Level == s.Current {
		s.pending, s.streak = "", 0
		return s.Current, ""
	}
	if v.Level != s.pending {
		s.pending, s.streak = v.Level, 1
		return s.Current, ""
	}
	s.streak++
	if s.streak < hysteresisStreak {
		return s.Current, ""
	}
	s.Current = v.Level
	s.pending, s.streak = "", 0
	return s.Current, v.Why
}
