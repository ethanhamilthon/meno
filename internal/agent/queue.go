package agent

import "sync"

// Queue holds what waits for the next step of a running turn: messages the
// user wrote meanwhile, and images the agent asked to see with view(). Others
// push; the runner takes them at the next step boundary. The zero value is ready.
type Queue struct {
	mu    sync.Mutex
	items []Message
}

func (q *Queue) Push(m Message) {
	q.mu.Lock()
	q.items = append(q.items, m)
	q.mu.Unlock()
}

// Take removes and returns everything queued.
func (q *Queue) Take() []Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := q.items
	q.items = nil
	return out
}

// TakeWritten removes and returns the messages the user wrote; images queued
// by view() stay.
func (q *Queue) TakeWritten() []Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out, keep []Message
	for _, m := range q.items {
		if m.Viewed {
			keep = append(keep, m)
		} else {
			out = append(out, m)
		}
	}
	q.items = keep
	return out
}

// Items is a copy of the messages the user wrote.
func (q *Queue) Items() []Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []Message
	for _, m := range q.items {
		if !m.Viewed {
			out = append(out, m)
		}
	}
	return out
}

// Len is the number of queued messages the user wrote.
func (q *Queue) Len() int { return len(q.Items()) }

// Viewed is the number of images queued by view().
func (q *Queue) Viewed() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, m := range q.items {
		if m.Viewed {
			n++
		}
	}
	return n
}
