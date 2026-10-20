package engine

import "meno/internal/proto"

// queuedText returns a non-nil list, including when the queue is empty.
func queuedText(s *runState) []string {
	messages := s.queue.Items()
	queued := make([]string, 0, len(messages))
	for _, message := range messages {
		queued = append(queued, message.Text)
	}
	return queued
}

// publishQueue keeps queue snapshots in session event order. The caller holds s.mu.
// Queue updates do not change global session status.
func (e *Engine) publishQueue(s *runState, data proto.InjectedData) {
	data.Queued = queuedText(s)
	if e.events != nil {
		e.events.Publish(s.session.ID, proto.EventInjected, data)
	}
}
