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

// SetSystem sets the system-event block of turn i, if it exists.
func (l *SafeLog) SetSystem(i int, system string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if i >= 0 && i < len(l.turns) {
		l.turns[i].System = system
	}
}

// SetRunOnLast attaches run stats to the last turn, if there is one.
func (l *SafeLog) SetRunOnLast(r RunStats) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n := len(l.turns); n > 0 {
		l.turns[n-1].Run = &r
	}
}
