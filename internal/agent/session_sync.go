package agent

import "sync"

type SafeLog struct {
	mu    sync.RWMutex
	turns []Turn
}

func (l *SafeLog) Append(t Turn) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.turns = append(l.turns, t)
}

func (l *SafeLog) Snapshot() []Turn {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Turn, len(l.turns))
	copy(out, l.turns)
	return out
}

func (l *SafeLog) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.turns)
}

func (l *SafeLog) Replace(turns []Turn) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.turns = turns
}
