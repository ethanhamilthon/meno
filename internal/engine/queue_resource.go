package engine

import "meno/internal/proto"

// queueResource lets a human client take back the messages that wait for the
// next step (a DELETE), so they can be edited and sent again.
func (e *Engine) queueResource(s *runState, method string) any {
	if method != "DELETE" {
		var texts []string
		for _, m := range s.queue.Items() {
			texts = append(texts, m.Text)
		}
		return map[string]any{"queued": texts}
	}
	items := s.queue.TakeWritten()
	// A recall changes what every client shows as waiting: publish it.
	e.publishQueue(s, proto.InjectedData{Kind: "recall", By: "user"})
	texts := make([]string, 0, len(items))
	for _, m := range items {
		texts = append(texts, m.Text)
	}
	return map[string]any{"queued": texts}
}
