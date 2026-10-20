package engine

import (
	"context"
	"time"

	"meno/internal/agent"
	"meno/internal/proto"
)

// cancelChildren stops the subagents the request owner started.
func (e *Engine) cancelChildren(parent, owner string) {
	recs, err := e.children.List(parent, true)
	if err != nil {
		return
	}
	for _, r := range recs {
		if owner != "" && r.Owner != owner {
			continue
		}
		if s, err := e.load(context.Background(), r.ID); err == nil {
			s.mu.Lock()
			if s.cancel != nil {
				s.cancel()
			}
			s.mu.Unlock()
		}
		r.Status, r.Ended = agent.ChildCancelled, time.Now().UTC()
		_ = e.children.Save(r)
		e.publish(parent, proto.EventAgentsChanged, []string{r.ID})
	}
}
