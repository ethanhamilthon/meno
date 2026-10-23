package agent

import (
	"database/sql"
	"encoding/json"
)

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// marshalOrEmpty is the JSON of v, or "" when empty is set.
func marshalOrEmpty(v any, empty bool) (string, error) {
	if empty {
		return "", nil
	}
	b, err := json.Marshal(v)
	return string(b), err
}

// writeSession saves the session in one transaction. The active turns are
// replaced wholesale; turns that compaction moved out of the active context
// are appended to the history table, which is never rewritten.
func writeSession(db *sql.DB, s *Session) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	pend, err := writeSessionTx(tx, s)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.ensureLog().ClearPending(len(pend.Archive), len(pend.Summaries))
	return nil
}

// writeSessionTx does the work of writeSession inside tx and returns what it
// appended to the history tables; the caller clears that after committing.
func writeSessionTx(tx *sql.Tx, s *Session) (pend Pending, err error) {
	pend = s.ensureLog().Pending()
	s.NextHID = pend.Next
	metrics, err := json.Marshal(s.Metrics)
	if err != nil {
		return pend, err
	}
	kind := s.Kind
	if kind == "" {
		kind = KindMain
	}
	if _, err = tx.Exec(`INSERT INTO sessions(id,title,cwd,provider,model,effort,summary,metrics,updated_at,kind,parent_id,fork_source,fork_boundary,next_hid,coverage,legacy,tool,no_ask,spawned_by,system_name,system_prompt)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET title=excluded.title, cwd=excluded.cwd, provider=excluded.provider, model=excluded.model,
		effort=excluded.effort, summary=excluded.summary, metrics=excluded.metrics, updated_at=excluded.updated_at,
		next_hid=excluded.next_hid, coverage=excluded.coverage, tool=excluded.tool,
		spawned_by=excluded.spawned_by, system_name=excluded.system_name, system_prompt=excluded.system_prompt`,
		s.ID, s.Title, s.Cwd, s.Provider, s.Model, s.Effort, s.Summary, string(metrics), ts(s.UpdatedAt),
		kind, s.ParentID, s.ForkSource, s.ForkBoundary, s.NextHID, s.Coverage, b2i(s.Legacy), s.toolName(), b2i(s.NoAsk), s.SpawnedBy, s.SystemName, s.SystemPrompt); err != nil {
		return pend, err
	}
	for _, t := range pend.Archive {
		t.Time = t.Time.UTC()
		b, merr := json.Marshal(t)
		if merr != nil {
			return pend, merr
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO history(session_id,hid,turn,archived_at) VALUES(?,?,?,?)", s.ID, t.HID, string(b), ts(s.UpdatedAt)); err != nil {
			return pend, err
		}
	}
	for _, r := range pend.Summaries {
		if _, err = tx.Exec("INSERT OR IGNORE INTO summaries(session_id,rev,summary,covers_through,created_at) VALUES(?,?,?,?,?)", s.ID, r.Rev, r.Summary, r.CoversThrough, ts(r.Time)); err != nil {
			return pend, err
		}
	}
	if _, err = tx.Exec("DELETE FROM turns WHERE session_id = ?", s.ID); err != nil {
		return pend, err
	}
	if _, err = tx.Exec("DELETE FROM requests WHERE session_id = ?", s.ID); err != nil {
		return pend, err
	}
	ins, err := tx.Prepare(`INSERT INTO turns(session_id,seq,role,text,thinking,call_id,code,output,err,replay,replay_group,time,system,attachments,kind,elapsed_ms,limit_ms,timed_out,run,hid,event_id,title)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return pend, err
	}
	defer ins.Close()
	for i, t := range s.Turns {
		var replay any
		if t.Replay != nil {
			b, merr := json.Marshal(t.Replay)
			if merr != nil {
				return pend, merr
			}
			replay = string(b)
		}
		attachments, merr := marshalOrEmpty(t.Attachments, len(t.Attachments) == 0)
		if merr != nil {
			return pend, merr
		}
		run, merr := marshalOrEmpty(t.Run, t.Run == nil)
		if merr != nil {
			return pend, merr
		}
		if _, err = ins.Exec(s.ID, i, t.Role, t.Text, t.Thinking, t.CallID, t.Code, t.Output, t.Err, replay, t.ReplayGroup, ts(t.Time),
			t.System, attachments, t.Kind, t.ElapsedMs, t.LimitMs, b2i(t.TimedOut), run, t.HID, t.EventID, t.Title); err != nil {
			return pend, err
		}
	}
	req, err := tx.Prepare("INSERT INTO requests(session_id,seq,provider,model,usage,cost,time,details) VALUES(?,?,?,?,?,?,?,?)")
	if err != nil {
		return pend, err
	}
	defer req.Close()
	records, configs := s.requestSnapshot()
	if err := writeRequestConfigs(tx, s.ID, configs); err != nil {
		return pend, err
	}
	for i, r := range records {
		usage, merr := json.Marshal(r.Usage)
		if merr != nil {
			return pend, merr
		}
		var cost any
		if r.Cost != nil {
			cost = *r.Cost
		}
		details, merr := marshalOrEmpty(r.Record, r.Record == nil)
		if merr != nil {
			return pend, merr
		}
		if _, err = req.Exec(s.ID, i, r.Provider, r.Model, string(usage), cost, ts(r.Time), details); err != nil {
			return pend, err
		}
	}
	return pend, nil
}
