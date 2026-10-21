package engine

import (
	"context"
	"errors"
	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/proto"
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
	s.live = proto.LiveState{Calls: []proto.CallState{}}
	if err != nil && !errors.Is(err, context.Canceled) {
		s.lastErr = err.Error()
	}
	saveErr := agent.SaveCompletion(e.home, s.session)
	queued := s.queue.Take()
	s.visibleQueue = nil
	requestID, metrics := s.requestID, s.metrics
	s.mu.Unlock()
	e.publishQueueCommitted(s)
	if saveErr != nil {
		e.publish(id, proto.EventError, proto.ErrorData{Message: saveErr.Error()})
	}
	e.publish(id, proto.EventSessionChanged, proto.ChangedData{Fields: []string{"run", "unread"}})
	e.publish(id, proto.EventStatus, proto.StatusData{Running: false, RequestID: requestID})
	if err != nil && errors.Is(err, context.Canceled) {
		e.publish(id, proto.EventAborted, proto.DoneData{RequestID: requestID})
	} else if err != nil {
		e.publish(id, proto.EventError, proto.ErrorData{RequestID: requestID, Message: err.Error()})
	} else {
		e.publish(id, proto.EventDone, proto.DoneData{RequestID: requestID, Metrics: metrics})
	}
	if len(queued) > 0 && err == nil {
		s.mu.Lock()
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
		e.publish(id, proto.EventStatus, proto.StatusData{Running: true, Phase: "preparing", RequestID: rid})
		e.launch(ctx, s, msg.Text, false)
	}
}
