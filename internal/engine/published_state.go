package engine

import (
	"context"
	"meno/internal/agent"
	"meno/internal/api"
)

// wireTurns detaches snapshot-visible history from the runner. The commit lock
// covers projection updates and their event cursors; mu protects the projection.
// No provider work is performed under commit. Runner mutations remain private
// until their next event publishes the corresponding history.
func wireTurns(turns []agent.Turn) []api.Turn {
	out := make([]api.Turn, 0, len(turns))
	for _, t := range turns {
		out = append(out, turnWire(t))
	}
	return out
}

// publishQueueCommitted is used by finish while it already holds commit.
func (e *Engine) publishQueueCommitted(s *runState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.publishQueue(s, api.InjectedData{})
}

// publishAgentTurns requires commit. A compacted/replaced history cannot be
// reconstructed by appending turns: tell observers to reload the projection.
func (e *Engine) publishAgentTurns(id string, turns []agent.Turn) {
	s, err := e.load(context.Background(), id)
	if err != nil {
		return
	}
	s.mu.Lock()
	visible := wireTurns(turns)
	from := s.published
	replaced := len(visible) < len(s.visibleTurns)
	for i := 0; i < min(len(visible), len(s.visibleTurns)); i++ {
		if visible[i].ID != s.visibleTurns[i].ID {
			replaced = true
			break
		}
	}
	if replaced || from > len(turns) {
		from = len(turns)
	}
	s.published = len(turns)
	s.visibleTurns = visible
	s.mu.Unlock()
	for _, t := range visible[from:] {
		e.publish(id, api.EventTurnAdded, api.TurnAddedData{Turn: t})
	}
	if replaced {
		e.publish(id, api.EventSessionChanged, api.ChangedData{Fields: []string{"history"}})
	}
}
