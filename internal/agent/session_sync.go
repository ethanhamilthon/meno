package agent

import (
	"sync"
	"time"
)

type SafeLog struct {
	mu    sync.RWMutex
	turns []Turn
	next  int64 // last assigned history ID
	// Compaction output not yet saved: see Pending.
	archive []Turn
	sums    []SummaryRev
}

// SummaryRev is one revision of the continuation summary. CoversThrough is the
// last history ID the summary replaces in the active context.
type SummaryRev struct {
	Rev           int
	Summary       string
	CoversThrough int64
	Time          time.Time
}

// Pending is what the next save must append to the history tables.
type Pending struct {
	Next      int64
	Archive   []Turn
	Summaries []SummaryRev
}

// NextHID is the last history ID assigned.
func (l *SafeLog) NextHID() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.next
}

func newSafeLog(turns []Turn, next int64) *SafeLog {
	l := &SafeLog{turns: turns, next: next}
	for _, t := range turns {
		l.next = max(l.next, t.HID)
	}
	for i := range l.turns {
		if l.turns[i].HID == 0 { // sessions saved before history IDs existed
			l.next++
			l.turns[i].HID = l.next
		}
	}
	return l
}

func (l *SafeLog) Append(t Turn) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t.HID == 0 {
		l.next++
		t.HID = l.next
	} else {
		l.next = max(l.next, t.HID)
	}
	l.turns = append(l.turns, t)
}

// Pending returns the history that compaction has queued for the next save.
func (l *SafeLog) Pending() Pending {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return Pending{Next: l.next, Archive: append([]Turn(nil), l.archive...), Summaries: append([]SummaryRev(nil), l.sums...)}
}

// ClearPending drops the first archived turns and summaries after a save.
func (l *SafeLog) ClearPending(archive, sums int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.archive = l.archive[min(archive, len(l.archive)):]
	l.sums = l.sums[min(sums, len(l.sums)):]
}

// Compact makes tail the active context and queues head and the summary for
// the history tables. Nothing is discarded.
func (l *SafeLog) Compact(head, tail []Turn, r SummaryRev) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n := len(head); n > 0 {
		r.CoversThrough = head[n-1].HID
	}
	l.archive = append(l.archive, head...)
	l.sums = append(l.sums, r)
	l.turns = tail
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
	for i := range turns {
		if turns[i].HID == 0 {
			l.next++
			turns[i].HID = l.next
		}
		l.next = max(l.next, turns[i].HID)
	}
	l.turns = turns
}

// AppendToLastOutput adds text to the tool output of the last turn.
func (l *SafeLog) AppendToLastOutput(text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n := len(l.turns); n > 0 {
		l.turns[n-1].Output += text
	}
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
