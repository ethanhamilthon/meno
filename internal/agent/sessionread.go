package agent

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"meno/internal/llm"
)

func readSession(db *sql.DB, id string) (*Session, error) {
	var s Session
	var metrics, updated, modes string
	var legacy int
	err := db.QueryRow("SELECT id,title,cwd,provider,model,effort,summary,metrics,updated_at,kind,parent_id,fork_source,fork_boundary,next_hid,coverage,legacy,modes FROM sessions WHERE id = ?", id).
		Scan(&s.ID, &s.Title, &s.Cwd, &s.Provider, &s.Model, &s.Effort, &s.Summary, &metrics, &updated, &s.Kind, &s.ParentID, &s.ForkSource, &s.ForkBoundary, &s.NextHID, &s.Coverage, &legacy, &modes)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	s.UpdatedAt = parseTS(updated)
	s.Legacy = legacy != 0
	s.Modes = strings.Fields(modes)
	if err := json.Unmarshal([]byte(metrics), &s.Metrics); err != nil {
		return nil, fmt.Errorf("session %s metrics: %w", id, err)
	}
	rows, err := db.Query("SELECT role,text,thinking,call_id,code,output,err,replay,replay_group,time,system,attachments,kind,elapsed_ms,limit_ms,timed_out,run,hid,event_id,title FROM turns WHERE session_id = ? ORDER BY seq", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Turn
		var replay sql.NullString
		var when, attachments, run string
		var timedOut int
		if err := rows.Scan(&t.Role, &t.Text, &t.Thinking, &t.CallID, &t.Code, &t.Output, &t.Err, &replay, &t.ReplayGroup, &when, &t.System, &attachments, &t.Kind, &t.ElapsedMs, &t.LimitMs, &timedOut, &run, &t.HID, &t.EventID, &t.Title); err != nil {
			return nil, err
		}
		t.TimedOut = timedOut != 0
		if run != "" {
			var rs RunStats
			// A damaged summary must not make the session unreadable: it only decorates.
			if json.Unmarshal([]byte(run), &rs) == nil {
				t.Run = &rs
			}
		}
		if attachments != "" {
			if err := json.Unmarshal([]byte(attachments), &t.Attachments); err != nil {
				return nil, fmt.Errorf("session %s attachments: %w", id, err)
			}
		}
		if replay.Valid {
			var m llm.Msg
			if err := json.Unmarshal([]byte(replay.String), &m); err != nil {
				return nil, fmt.Errorf("session %s replay: %w", id, err)
			}
			t.Replay = &m
		}
		t.Time = parseTS(when)
		s.Turns = append(s.Turns, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rq, err := db.Query("SELECT provider,model,usage,cost,time FROM requests WHERE session_id = ? ORDER BY seq", id)
	if err != nil {
		return nil, err
	}
	defer rq.Close()
	for rq.Next() {
		var r RequestUsage
		var usage, when string
		var cost sql.NullFloat64
		if err := rq.Scan(&r.Provider, &r.Model, &usage, &cost, &when); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(usage), &r.Usage); err != nil {
			return nil, err
		}
		if cost.Valid {
			c := cost.Float64
			r.Cost = &c
		}
		r.Time = parseTS(when)
		s.Requests = append(s.Requests, r)
	}
	return &s, rq.Err()
}

func listSessions(db *sql.DB, limit int) ([]SessionInfo, error) {
	rows, err := db.Query(`SELECT s.id, s.title, s.cwd, s.updated_at,
		(SELECT COUNT(1) FROM turns t WHERE t.session_id = s.id),
		(SELECT COUNT(1) FROM turns t WHERE t.session_id = s.id AND (t.code <> '' OR t.call_id <> '')),
		s.provider, s.model, s.effort
		FROM sessions s WHERE s.kind = 'main' ORDER BY s.updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionInfo
	for rows.Next() {
		var i SessionInfo
		var updated string
		if err := rows.Scan(&i.ID, &i.Title, &i.Cwd, &updated, &i.Turns, &i.JSRuns, &i.Provider, &i.Model, &i.Effort); err != nil {
			return nil, err
		}
		i.UpdatedAt = parseTS(updated)
		out = append(out, i)
	}
	return out, rows.Err()
}
