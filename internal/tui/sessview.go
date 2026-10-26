package tui

import (
	"strings"
	"time"

	"meno/internal/api"
	"meno/internal/render"
)

// sessView is a session as the TUI knows it: a copy of what the daemon told.
// The daemon owns the real session; this is only for drawing and choosing.
type sessView struct {
	Completed, ReadThrough int64
	id                     string
	Title                  string
	Cwd                    string
	Provider, Model        string
	Effort                 string
	turns                  []render.Turn
	UpdatedAt              time.Time
	hasSummary             bool
	revision               uint64
	dirtyFrom              int
	byID                   map[string]int
	indexed                int
}

// ID is the session id.
func (s *sessView) ID() string { return s.id }

// Len is the number of turns shown.
func (s *sessView) Len() int { return len(s.turns) }

// Turns are the turns to draw.
func (s *sessView) Turns() []render.Turn { return s.turns }

// DisplayTitle is the title for lists and the composer rule.
func (s *sessView) DisplayTitle() string {
	if t := strings.TrimSpace(s.Title); t != "" {
		return t
	}
	return "untitled"
}

// setTitle names the session on screen; the daemon stores the real name.
func (s *sessView) setTitle(t string) { s.Title = t }

// load replaces what is shown with a snapshot.
func (s *sessView) load(sn api.Snapshot) {
	s.Completed, s.ReadThrough = sn.Session.Completed, sn.Session.ReadThrough
	s.id = sn.Session.ID
	s.Title = sn.Session.Name
	if s.Title == "untitled" {
		s.Title = ""
	}
	s.Cwd, s.Provider, s.Model, s.Effort = sn.Cwd, sn.Provider, sn.Model, sn.Effort
	s.resetTurns(sn.Turns)
	if sn.Session.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, sn.Session.UpdatedAt); err == nil {
			s.UpdatedAt = t
		}
	}
}

// changed marks the earliest suffix needing layout, across a batch of updates.
func (s *sessView) changed(i int) { s.revision++; s.dirtyFrom = min(s.dirtyFrom, i) }
func (s *sessView) indexTurns() {
	if s.byID != nil && s.indexed == len(s.turns) {
		return
	}
	s.byID = map[string]int{}
	for i, t := range s.turns {
		if t.ID != "" && t.ID != "0" {
			s.byID[t.ID] = i
		}
	}
	s.indexed = len(s.turns)
}
func (s *sessView) resetTurns(turns []api.Turn) {
	s.turns = nil
	s.byID = map[string]int{}
	s.indexed = 0
	s.changed(0)
	for _, t := range turns {
		s.upsert(t)
	}
}
func (s *sessView) upsert(t api.Turn) {
	s.indexTurns()
	v := turnFromWire(t)
	if i, ok := s.byID[v.ID]; v.ID != "" && ok {
		s.turns[i] = v
		s.changed(i)
		return
	}
	i := len(s.turns)
	s.turns = append(s.turns, v)
	if v.ID != "" {
		s.byID[v.ID] = i
	}
	s.indexed = len(s.turns)
	s.changed(i)
}
func (s *sessView) add(t api.Turn) bool {
	s.indexTurns()
	if _, ok := s.byID[t.ID]; t.ID != "" && t.ID != "0" && ok {
		return false
	}
	s.upsert(t)
	return true
}

// turnFromWire converts a wire turn into what the transcript draws.
func turnFromWire(t api.Turn) render.Turn {
	v := render.Turn{ID: t.ID, Role: t.Role, Kind: t.Kind, Text: t.Content, Thinking: t.Thinking, Err: t.Err,
		ElapsedMs: t.ElapsedMS, TimedOut: t.TimedOut}
	if v.ID == "0" {
		v.ID = ""
	}
	for _, p := range t.Parts {
		if p.Type == "call" && p.Call != nil {
			v.CallID, v.Code, v.Title, v.Output = p.Call.CallID, p.Call.Command, p.Call.Title, p.Call.Output
			if v.ElapsedMs == 0 {
				v.ElapsedMs = p.Call.ElapsedMS
			}
		}
		if p.Type == "thinking" && v.Thinking == "" {
			v.Thinking = p.Text
		}
		if p.Type == "text" && v.Text == "" {
			v.Text = p.Text
		}
	}
	if r := t.Run; r != nil {
		v.Run = &render.RunStats{Status: r.Status, ElapsedMs: r.ElapsedMS, Turns: r.Turns, ToolCalls: r.ToolCalls, ToolMs: r.ToolMS,
			In: r.In, Out: r.Out, Cached: r.Cached, CacheWrite: r.CacheWrite, CacheKnown: r.CacheKnown, Cost: r.Cost, Partial: r.Partial, Err: r.Err}
	}
	return v
}
