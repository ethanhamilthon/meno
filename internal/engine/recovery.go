package engine

import (
	"meno/internal/agent"
	"meno/internal/db"
	"time"
)

// recoverSession runs when this process becomes the owner of a stored session.
// The process that ran its tasks is gone: the tasks are marked interrupted and a
// request that was running gets a closing turn. Nothing is restarted.
func (e *Engine) recoverSession(id string) (changed bool, err error) {
	if err = e.tasks.MarkInterrupted(id); err != nil {
		return false, err
	}
	d, err := db.Open(e.home)
	if err != nil {
		return false, err
	}
	var n int
	if err = d.QueryRow("SELECT COUNT(1) FROM tasks WHERE session_id = ? AND status = 'interrupted'", id).Scan(&n); err != nil || n == 0 {
		return false, err
	}
	s, err := agent.LoadSession(e.home, id)
	if err != nil {
		return false, nil
	}
	turns := s.Snapshot()
	if len(turns) > 0 && (turns[len(turns)-1].Run == nil || turns[len(turns)-1].Run.Status == "") {
		s.Append(agent.Turn{Role: "assistant", Text: "[interrupted: meno restarted while this request was running]", Time: time.Now().UTC()})
		s.SetRun(agent.RunStats{Status: agent.RunStop, Err: "interrupted by restart"})
		return true, agent.SaveCompletion(e.home, s, false)
	}
	return false, nil
}
