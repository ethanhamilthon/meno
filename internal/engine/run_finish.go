package engine

import (
	"context"
	"errors"
	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/llm"
	"time"
)

func (e *Engine) finish(s *runState, err error) {
	id := s.session.ID
	s.commit.Lock()
	defer s.commit.Unlock()
	e.publishAgentTurns(id, s.session.Snapshot())
	s.mu.Lock()
	s.running = false
	s.phase = ""
	s.cancel = nil
	s.retry = nil
	s.live = api.LiveState{Calls: []api.CallState{}}
	if err != nil && !errors.Is(err, context.Canceled) {
		s.lastErr = err.Error()
	}
	queued := s.queue.Take()
	// Undrained written messages belong to the follow-up run, not this one.
	k := 0
	for _, m := range queued {
		if !m.Viewed {
			k++
		}
	}
	split := len(s.authors) - k
	countDone := lastAuthorCounts(s.authors[:split])
	leftoverAuthors := s.authors[split:]
	s.authors = nil
	saveErr := agent.SaveCompletion(e.home, s.session, countDone)
	s.visibleQueue = nil
	requestID, metrics := s.requestID, s.metrics
	s.mu.Unlock()
	if turns := s.session.Snapshot(); len(turns) > 0 {
		s.mu.Lock()
		s.visibleTurns = wireTurns(turns)
		s.mu.Unlock()
		e.publish(id, api.EventTurnUpdated, api.TurnAddedData{Turn: turnWire(turns[len(turns)-1])})
	}
	e.publishQueueCommitted(s)
	if saveErr != nil {
		e.publish(id, api.EventError, api.ErrorData{Message: saveErr.Error()})
	}
	e.publishSessionChanged(s, "run", "unread")
	e.publish(id, api.EventStatus, api.StatusData{Running: false, RequestID: requestID})
	if err != nil && errors.Is(err, context.Canceled) {
		e.publish(id, api.EventAborted, api.DoneData{Counted: countDone, RequestID: requestID})
	} else if err != nil {
		e.publish(id, api.EventError, api.ErrorData{RequestID: requestID, Message: err.Error()})
	} else {
		e.publish(id, api.EventDone, api.DoneData{Counted: countDone, RequestID: requestID, Metrics: metrics})
	}
	if len(queued) > 0 && err == nil {
		s.mu.Lock()
		s.authors = leftoverAuthors
		msg := agent.Combine(queued)
		s.session.Append(agent.Turn{Role: llm.RoleUser, Text: msg.Text, Attachments: msg.Attachments, Time: time.Now().UTC()})
		_ = e.save(s)
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		s.running = true
		s.phase = "preparing"
		s.requestID = agent.NewID()
		rid := s.requestID
		turns := s.session.Snapshot()
		s.mu.Unlock()
		e.publishAgentTurns(id, turns)
		e.publish(id, api.EventStatus, api.StatusData{Running: true, Phase: "preparing", RequestID: rid})
		e.launch(ctx, s, msg.Text, false)
	}
}
