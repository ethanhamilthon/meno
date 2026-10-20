package engine

import (
	"database/sql"
	"meno/internal/agent"
	"time"
)

func recoverInterrupted(home string, d *sql.DB) error {
	rows, err := d.Query("SELECT id FROM sessions WHERE id IN (SELECT session_id FROM tasks WHERE status='interrupted') AND id NOT IN (SELECT id FROM agents)")
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, id := range ids {
		s, er := agent.LoadSession(home, id)
		if er != nil {
			continue
		}
		turns := s.Snapshot()
		if len(turns) > 0 && (turns[len(turns)-1].Run == nil || turns[len(turns)-1].Run.Status == "") {
			s.Append(agent.Turn{Role: "assistant", Text: "[interrupted: meno restarted while this request was running]", Time: now})
			if er = agent.SaveSession(home, s); er != nil {
				return er
			}
		}
	}
	return nil
}
