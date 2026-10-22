package engine

import (
	"meno/internal/agent"
	"meno/internal/api"
	"strings"
)

func (e *Engine) agentEvent(s *runState, ev agent.Ev) {
	id := s.session.ID
	s.commit.Lock()
	defer s.commit.Unlock()
	s.mu.Lock()
	switch ev.Kind {
	case agent.EvText:
		s.live.Text += ev.Text
	case agent.EvThink:
		s.live.Thinking += ev.Thinking
	case agent.EvCode:
		c := findCall(&s.live, ev.CallID)
		old := c.Delta
		delta := ev.Code
		if strings.HasPrefix(ev.Code, old) {
			delta = strings.TrimPrefix(ev.Code, old)
		}
		c.Delta = ev.Code
		c.Command = ev.Code
		s.mu.Unlock()
		if delta != "" {
			e.publish(id, api.EventCallDelta, api.CallDeltaData{CallID: ev.CallID, Delta: delta})
		}
		return
	case agent.EvJSStart:
		c := findCall(&s.live, ev.CallID)
		c.Command = ev.Code
		c.Title = ev.Title
	case agent.EvJSEnd:
		c := findCall(&s.live, ev.CallID)
		c.Output = ev.Output
		c.Exit = exitFromOutput(ev.Output)
		c.ElapsedMS = ev.Elapsed.Milliseconds()
		c.TaskID = ev.TaskID
		if ev.Err != "" {
			c.Exit = exitCode(ev.Err)
		}
	case agent.EvRequest:
		if ev.Req != nil {
			s.lastRequest = *ev.Req
			s.lastRequests = append(s.lastRequests, *ev.Req)
			if len(s.lastRequests) > 2 {
				s.lastRequests = s.lastRequests[len(s.lastRequests)-2:]
			}
		}
		s.phase = "thinking"
	case agent.EvRetry:
		s.live = api.LiveState{Calls: []api.CallState{}}
		s.retry = &api.RetryState{Attempt: ev.Attempt, InMS: ev.Delay.Milliseconds(), Reason: ev.Err}
	case agent.EvUsage:
		s.metrics.InputTokens += int64(ev.Usage.In)
		s.metrics.OutputTokens += int64(ev.Usage.Out)
		s.session.Metrics.LastUsage = ev.Usage
		s.session.Metrics.Usage.In += ev.Usage.In
		s.session.Metrics.Usage.Out += ev.Usage.Out
		s.session.Metrics.Usage.Cached += ev.Usage.Cached
		s.session.Metrics.Usage.CacheWrite += ev.Usage.CacheWrite
		s.session.Metrics.Usage.CacheReported = s.session.Metrics.Usage.CacheReported || ev.Usage.CacheReported
		priced := false
		if p, ok := e.provider(s.session.Provider); ok {
			s.session.Metrics.Context = ev.Usage.In
			if mi, ok := p.Lookup(e.catalogView(), s.session.Model); ok && mi.CanPrice(ev.Usage) {
				priced = true
				cost := mi.Cost(ev.Usage)
				s.metrics.Cost += cost
				s.session.Metrics.Spent += cost
			}
		}
		if !priced {
			s.session.Metrics.Unpriced = true
		}
	case agent.EvError:
		s.lastErr = ev.Err
	case agent.EvAborted:
		s.live = api.LiveState{Calls: []api.CallState{}}
	}
	if ev.Kind == agent.EvJSEnd {
		s.live.Calls = removeCall(s.live.Calls, ev.CallID)
	}
	if ev.Kind == agent.EvInjected {
		e.publishQueue(s, api.InjectedData{})
	}
	// Fragments are live-only. Do not copy history or enter persistence on the
	// hot streaming path, even when runner-owned history has changed.
	if isFragment(ev.Kind) {
		s.mu.Unlock()
		e.translate(id, ev)
		return
	}
	if persistEvent(ev.Kind, s.session.Len() > s.published) {
		_ = e.save(s)
	}
	turns := s.session.Snapshot()
	// History is a published projection: runner appends happen before Emit,
	// so snapshots must not read the mutable runner history mid-publication.
	from := min(s.published, len(turns))
	for _, t := range turns[from:] {
		if t.Role == "assistant" {
			s.live.Text, s.live.Thinking = "", ""
		}
	}
	s.mu.Unlock()
	e.publishAgentTurns(id, turns)
	e.translate(id, ev)
}

func findCall(l *api.LiveState, id string) *api.CallState {
	if l.Calls == nil {
		l.Calls = []api.CallState{}
	}
	for i := range l.Calls {
		if l.Calls[i].CallID == id {
			return &l.Calls[i]
		}
	}
	l.Calls = append(l.Calls, api.CallState{CallID: id})
	return &l.Calls[len(l.Calls)-1]
}
func removeCall(a []api.CallState, id string) []api.CallState {
	o := a[:0]
	for _, c := range a {
		if c.CallID != id {
			o = append(o, c)
		}
	}
	return o
}
func exitCode(string) *int { v := 1; return &v }

func (e *Engine) translate(id string, ev agent.Ev) {
	switch ev.Kind {
	case agent.EvText:
		e.publish(id, api.EventTextDelta, api.TextDeltaData{Text: ev.Text})
	case agent.EvThink:
		e.publish(id, api.EventThinkDelta, api.TextDeltaData{Text: ev.Thinking})
	case agent.EvJSStart:
		e.publish(id, api.EventCallStart, api.CallStartData{CallID: ev.CallID, Command: ev.Code, Title: ev.Title})
	case agent.EvJSEnd:
		exit := 0
		if ev.Err != "" {
			exit = 1
		}
		if code, ok := e.bashExit(id, ev.Output); ok && ev.TaskID == "" {
			exit = code
		}
		e.publish(id, api.EventCallEnd, api.CallEndData{CallID: ev.CallID, Output: ev.Output, Exit: exit, ElapsedMS: ev.Elapsed.Milliseconds(), TaskID: ev.TaskID})
	case agent.EvRequest:
		d := api.RequestData{Provider: ev.Provider.Name, Model: ev.Model}
		if ev.Req != nil {
			d.ContextTokens = requestTokens(ev.Req)
		}
		e.publish(id, api.EventRequest, d)
		e.publish(id, api.EventStatus, api.StatusData{Running: true, Phase: "thinking", RequestID: e.currentRequest(id)})
	case agent.EvRetry:
		e.publish(id, api.EventRetry, api.RetryData{Attempt: ev.Attempt, Max: ev.Max, InMS: ev.Delay.Milliseconds(), Reason: ev.Err})
	case agent.EvUsage:
		e.publish(id, api.EventUsage, e.usageData(id, ev))
	case agent.EvError:
		e.publish(id, api.EventError, api.ErrorData{Message: ev.Err})
		// Done/aborted are published once by finish, after final stats and attention.
	}
}
