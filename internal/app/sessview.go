package app

import (
	"strings"
	"time"

	"meno/internal/proto"
	"meno/internal/ui"
)

// sessView is a session as the TUI knows it: a copy of what the daemon told.
// The daemon owns the real session; this is only for drawing and choosing.
type sessView struct {
	id, summaryNote string
	Title           string
	Cwd             string
	Provider, Model string
	Effort          string
	Modes           []string
	turns           []ui.Turn
	UpdatedAt       time.Time
	hasSummary      bool
}

// ID is the session id.
func (s *sessView) ID() string { return s.id }

// Len is the number of turns shown.
func (s *sessView) Len() int { return len(s.turns) }

// Turns are the turns to draw.
func (s *sessView) Turns() []ui.Turn { return s.turns }

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
func (s *sessView) load(sn proto.Snapshot) {
	s.id = sn.Session.ID
	s.Title = sn.Session.Name
	if s.Title == "untitled" {
		s.Title = ""
	}
	s.Cwd, s.Provider, s.Model, s.Effort = sn.Cwd, sn.Provider, sn.Model, sn.Effort
	s.Modes = append([]string(nil), sn.Modes...)
	s.turns = s.turns[:0]
	for _, t := range sn.Turns {
		s.turns = append(s.turns, turnFromWire(t))
	}
	if sn.Session.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, sn.Session.UpdatedAt); err == nil {
			s.UpdatedAt = t
		}
	}
}

// add appends a turn the daemon stored.
func (s *sessView) add(t proto.Turn) { s.turns = append(s.turns, turnFromWire(t)) }

// turnFromWire converts a wire turn into what the transcript draws.
func turnFromWire(t proto.Turn) ui.Turn {
	v := ui.Turn{ID: t.ID, Role: t.Role, Kind: t.Kind, Text: t.Content, Mode: t.Mode, Thinking: t.Thinking, Err: t.Err,
		ElapsedMs: t.ElapsedMS, LimitMs: t.LimitMS, TimedOut: t.TimedOut}
	if v.ID == "0" {
		v.ID = ""
	}
	for _, p := range t.Parts {
		if p.Type == "call" && p.Call != nil {
			v.CallID, v.Code, v.Title, v.Output = p.Call.CallID, p.Call.Command, p.Call.Title, p.Call.Output
			if v.ElapsedMs == 0 {
				v.ElapsedMs, v.LimitMs, v.TimedOut = p.Call.ElapsedMS, p.Call.LimitMS, p.Call.TimedOut
			}
		}
		if p.Type == "thinking" && v.Thinking == "" {
			v.Thinking = p.Text
		}
		if p.Type == "text" && v.Text == "" {
			v.Text = p.Text
		}
	}
	for _, a := range t.Attachments {
		v.Attachments = append(v.Attachments, ui.Attachment{N: a.N, Mime: a.Mime, SHA: a.SHA, W: a.W, H: a.H})
	}
	if r := t.Run; r != nil {
		v.Run = &ui.RunStats{Status: r.Status, ElapsedMs: r.ElapsedMS, Turns: r.Turns, ToolCalls: r.ToolCalls, ToolMs: r.ToolMS,
			In: r.In, Out: r.Out, Cached: r.Cached, Cost: r.Cost, Err: r.Err}
	}
	return v
}
