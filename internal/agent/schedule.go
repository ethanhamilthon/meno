package agent

import (
	"errors"
	"time"
)

func timeNow() time.Time { return time.Now() }

// Scheduled is a compaction an agent asked for with session.compact: it is
// applied after the current tool batch, never in the middle of it.
type Scheduled struct {
	Summary string
	Keep    int
}

// ErrAlreadyScheduled is returned for a second session.compact in one batch.
var ErrAlreadyScheduled = errors.New("a compaction is already scheduled in this tool batch")

// ScheduleCompaction records a compaction to apply after the current batch.
func (s *Session) ScheduleCompaction(summary string, keep int) error {
	if summary == "" {
		return errors.New("summary must not be empty")
	}
	if keep < 0 {
		return errors.New("saveLastNMessages must not be negative")
	}
	l := s.ensureLog()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sched != nil {
		return ErrAlreadyScheduled
	}
	l.sched = &Scheduled{Summary: summary, Keep: keep}
	return nil
}

// ApplyScheduled applies a scheduled compaction. The current replay group (the
// batch that scheduled it) and at least Keep messages stay in the active
// context, expanded so that no replay group is split. It returns how many
// messages stayed and whether anything was scheduled. The originals move to the
// history tables; nothing is deleted.
func (s *Session) ApplyScheduled() (kept int, ok bool) {
	l := s.ensureLog()
	l.mu.Lock()
	sch := l.sched
	l.sched = nil
	if sch == nil {
		l.mu.Unlock()
		return 0, false
	}
	turns := l.turns
	cut := len(turns) - sch.Keep
	if n := len(turns); n > 0 {
		// The batch that scheduled this is the last replay group: it stays.
		first := n - 1
		for first > 0 && turns[n-1].ReplayGroup != "" && turns[first-1].ReplayGroup == turns[n-1].ReplayGroup {
			first--
		}
		cut = min(cut, first)
	}
	for cut > 0 && turns[cut].ReplayGroup != "" && turns[cut-1].ReplayGroup == turns[cut].ReplayGroup {
		cut--
	}
	l.mu.Unlock()
	if cut <= 0 {
		return len(turns), true
	}
	head, tail := append([]Turn(nil), turns[:cut]...), append([]Turn(nil), turns[cut:]...)
	l.Compact(head, tail, SummaryRev{Rev: int(time.Now().UnixNano() / 1000), Summary: sch.Summary, Time: time.Now()})
	s.Summary = sch.Summary
	return len(tail), true
}
