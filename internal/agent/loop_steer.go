package agent

import "meno/internal/llm"

// inject adds the queued user messages to the session as one turn.
func (r *Runner) inject(s *Session) {
	if r.D.Events != nil {
		evs := r.D.Events.TakeEvents()
		for _, e := range evs {
			s.Append(eventTurn(e))
		}
		if len(evs) > 0 {
			r.emit(s, Ev{Kind: EvInjected})
		}
	}
	if r.D.Queue == nil {
		return
	}
	items := r.D.Queue.Take()
	if len(items) == 0 {
		return
	}
	var viewed, written []Message
	for _, m := range items {
		if m.Viewed {
			viewed = append(viewed, m)
		} else {
			written = append(written, m)
		}
	}
	if len(viewed) > 0 {
		m := Combine(viewed)
		s.Append(Turn{Role: llm.RoleUser, Kind: KindView, Text: m.Text, Attachments: m.Attachments})
	}
	if len(written) > 0 {
		m := Combine(written)
		s.Append(Turn{Role: llm.RoleUser, Text: m.Text, Attachments: m.Attachments})
	}
	r.emit(s, Ev{Kind: EvInjected})
}
