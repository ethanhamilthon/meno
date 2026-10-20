package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/proto"
	"strings"
)

func (e *Engine) agentEvent(s *runState, ev agent.Ev) {
	id := s.session.ID
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
			e.publish(id, proto.EventCallDelta, proto.CallDeltaData{CallID: ev.CallID, Delta: delta})
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
		s.retry = &proto.RetryState{Attempt: ev.Attempt, InMS: ev.Delay.Milliseconds(), Reason: ev.Err}
	case agent.EvUsage:
		s.metrics.InputTokens += int64(ev.Usage.In)
		s.metrics.OutputTokens += int64(ev.Usage.Out)
		s.session.Metrics.LastUsage = ev.Usage
		s.session.Metrics.Usage.In += ev.Usage.In
		s.session.Metrics.Usage.Out += ev.Usage.Out
		s.session.Metrics.Usage.Cached += ev.Usage.Cached
		s.session.Metrics.Usage.CacheWrite += ev.Usage.CacheWrite
		if p, ok := e.provider(s.session.Provider); ok {
			if mi, ok := p.Lookup(e.catalogView(), s.session.Model); ok {
				s.session.Metrics.Context = mi.Context
			}
			if mi, ok := p.Lookup(e.catalogView(), s.session.Model); ok && mi.CanPrice(ev.Usage) {
				cost := mi.Cost(ev.Usage)
				s.metrics.Cost += cost
				s.session.Metrics.Spent += cost
			}
		}
	case agent.EvError:
		s.lastErr = ev.Err
	case agent.EvAborted:
		s.live = proto.LiveState{Calls: []proto.CallState{}}
	}
	if ev.Kind == agent.EvJSEnd {
		s.live.Calls = removeCall(s.live.Calls, ev.CallID)
	}
	if ev.Kind == agent.EvInjected {
		e.publishQueue(s, proto.InjectedData{})
	}
	_ = e.save(s)
	turns := s.session.Snapshot()
	s.mu.Unlock()
	e.publishAgentTurns(id, turns)
	e.translate(id, ev)
}

func findCall(l *proto.LiveState, id string) *proto.CallState {
	if l.Calls == nil {
		l.Calls = []proto.CallState{}
	}
	for i := range l.Calls {
		if l.Calls[i].CallID == id {
			return &l.Calls[i]
		}
	}
	l.Calls = append(l.Calls, proto.CallState{CallID: id})
	return &l.Calls[len(l.Calls)-1]
}
func removeCall(a []proto.CallState, id string) []proto.CallState {
	o := a[:0]
	for _, c := range a {
		if c.CallID != id {
			o = append(o, c)
		}
	}
	return o
}
func exitCode(string) *int { v := 1; return &v }

func (e *Engine) publishAgentTurns(id string, turns []agent.Turn) {
	s, err := e.load(context.Background(), id)
	if err != nil {
		return
	}
	s.mu.Lock()
	from := s.published
	if from > len(turns) {
		from = len(turns)
	}
	s.published = len(turns)
	s.mu.Unlock()
	for _, t := range turns[from:] {
		e.publish(id, proto.EventTurnAdded, proto.TurnAddedData{Turn: turnWire(t)})
	}
}

func (e *Engine) translate(id string, ev agent.Ev) {
	switch ev.Kind {
	case agent.EvText:
		e.publish(id, proto.EventTextDelta, proto.TextDeltaData{Text: ev.Text})
	case agent.EvThink:
		e.publish(id, proto.EventThinkDelta, proto.TextDeltaData{Text: ev.Thinking})
	case agent.EvJSStart:
		e.publish(id, proto.EventCallStart, proto.CallStartData{CallID: ev.CallID, Command: ev.Code, Title: ev.Title})
	case agent.EvJSEnd:
		exit := 0
		if ev.Err != "" {
			exit = 1
		}
		if code, ok := e.bashExit(id, ev.Output); ok && ev.TaskID == "" {
			exit = code
		}
		e.publish(id, proto.EventCallEnd, proto.CallEndData{CallID: ev.CallID, Output: ev.Output, Exit: exit, ElapsedMS: ev.Elapsed.Milliseconds(), TaskID: ev.TaskID})
	case agent.EvRequest:
		d := proto.RequestData{Provider: ev.Provider.Name, Model: ev.Model}
		if ev.Req != nil {
			d.ContextTokens = requestTokens(ev.Req)
		}
		e.publish(id, proto.EventRequest, d)
		e.publish(id, proto.EventStatus, proto.StatusData{Running: true, Phase: "thinking"})
	case agent.EvRetry:
		e.publish(id, proto.EventRetry, proto.RetryData{Attempt: ev.Attempt, Max: ev.Max, InMS: ev.Delay.Milliseconds(), Reason: ev.Err})
	case agent.EvUsage:
		e.publish(id, proto.EventUsage, e.usageData(id, ev))
	case agent.EvAborted:
		e.publish(id, proto.EventAborted, map[string]any{})
	case agent.EvError:
		e.publish(id, proto.EventError, proto.ErrorData{Message: ev.Err})
	case agent.EvDone:
		s, err := e.load(context.Background(), id)
		if err == nil {
			s.mu.Lock()
			metrics := s.metrics
			requestID := s.requestID
			s.mu.Unlock()
			e.publish(id, proto.EventDone, proto.DoneData{RequestID: requestID, Metrics: metrics})
		}
	}
}
