package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/proto"
)

func childEventID(id, kind string, gen int) string {
	return fmt.Sprintf("agent-%s-%s-g%d", id, kind, gen)
}

func eventSnippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = s[:n] + "… [cut; the full text is in: meno agents result ID]"
	}
	return s
}

func lastAssistantText(s *agent.Session) string {
	turns := s.Snapshot()
	for i := len(turns) - 1; i >= 0; i-- {
		if t := turns[i]; t.Role == llm.RoleAssistant && t.Code == "" && strings.TrimSpace(t.Text) != "" {
			return strings.TrimSpace(t.Text)
		}
	}
	return ""
}

// childFinished records how one execution generation of a subagent ended and
// tells the parent through a steering event.
func (e *Engine) childFinished(s *runState, runErr error) {
	parent := s.session.ParentID
	rec, err := e.children.Get(parent, s.session.ID)
	if err != nil {
		return
	}
	s.mu.Lock()
	blocked := s.block
	s.block = ""
	s.mu.Unlock()
	switch {
	case rec.Status == agent.ChildCancelled:
	case errors.Is(runErr, agent.ErrHalted):
		rec.Status, rec.BlockReason = agent.ChildBlocked, blocked
	case runErr != nil && !errors.Is(runErr, context.Canceled):
		rec.Status, rec.Error = agent.ChildFailed, runErr.Error()
	case errors.Is(runErr, context.Canceled):
		rec.Status = agent.ChildCancelled
	default:
		rec.Status = agent.ChildCompleted
		res := lastAssistantText(s.session)
		rec.Result = &res
	}
	rec.Ended = time.Now().UTC()
	_ = e.children.Save(rec)
	var text, kind string
	switch rec.Status {
	case agent.ChildCompleted:
		kind, text = "completed", "Agent "+rec.ID+" completed. Result:\n"+eventSnippet(*rec.Result, 3000)
	case agent.ChildFailed:
		kind, text = "failed", "Agent "+rec.ID+" failed: "+eventSnippet(rec.Error, 1000)
	case agent.ChildBlocked:
		kind, text = "blocked", "Agent "+rec.ID+" is blocked and waits for your reply (meno agents send): "+eventSnippet(rec.BlockReason, 1500)
	}
	if kind != "" {
		e.enqueueEvent(agent.Event{ID: childEventID(rec.ID, kind, rec.Gen), Session: parent, Source: agent.SourceChild, From: rec.ID,
			Kind: kind, Owner: rec.Owner, Text: text, Refs: []string{"child:" + rec.ID}}, false)
	}
	e.publish(parent, proto.EventAgentsChanged, []string{rec.ID})
}
