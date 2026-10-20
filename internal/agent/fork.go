package agent

import (
	"database/sql"
	"fmt"
	"os"
	"time"
)

// LegacyForkWarning is returned with a fork of a session that predates the
// versioned state journal: only the conversation could be carried over.
const LegacyForkWarning = "this session predates versioned state; the fork keeps the conversation only, with empty state"

// SafeBoundary expands the history ID hid to the end of its replay group, so a
// fork never splits an assistant message from its tool results. It returns 0
// when hid is not in turns.
func SafeBoundary(turns []Turn, hid int64) int64 {
	for i, t := range turns {
		if t.HID != hid {
			continue
		}
		end := t.HID
		for j := i + 1; t.ReplayGroup != "" && j < len(turns) && turns[j].ReplayGroup == t.ReplayGroup; j++ {
			end = turns[j].HID
		}
		return end
	}
	return 0
}

// AllTurns is the complete source history of a session, oldest first.
func AllTurns(dir, id string) ([]Turn, error) {
	d, err := openDB(dir)
	if err != nil {
		return nil, err
	}
	return allTurns(d, id)
}

func allTurns(d *sql.DB, id string) ([]Turn, error) {
	return readHistory(d, id, 0, 1<<30)
}

// ForkSession creates a new independent main session from the history of
// srcID up to and including the turn hid (expanded to a whole replay group).
// It copies the history with its IDs, the summary that was valid at that point
// (never one that contains later facts), the model settings, and the versioned
// state as it was at the boundary. cwd overrides the working directory when
// not empty. The returned warning is non-empty when the source is a legacy
// session. No model is called; nothing of the source is modified.
func ForkSession(dir, srcID string, hid int64, cwd string) (*Session, string, error) {
	d, err := openDB(dir)
	if err != nil {
		return nil, "", err
	}
	src, err := readSession(d, srcID)
	if err != nil {
		return nil, "", err
	}
	all, err := allTurns(d, srcID)
	if err != nil {
		return nil, "", err
	}
	boundary := SafeBoundary(all, hid)
	if boundary == 0 {
		return nil, "", fmt.Errorf("session %s has no message with history ID %d", srcID, hid)
	}
	if cwd == "" {
		cwd = src.Cwd
	}
	if st, err := os.Stat(cwd); err != nil || !st.IsDir() {
		return nil, "", fmt.Errorf("working directory %q does not exist; choose a directory for the fork", cwd)
	}
	revs, err := summaryRevs(d, srcID, boundary)
	if err != nil {
		return nil, "", err
	}
	fork := &Session{
		ID: NewID(), Title: src.Title, Cwd: cwd, Provider: src.Provider, Tool: src.Tool, Model: src.Model, Effort: src.Effort,
		UpdatedAt: time.Now(), Kind: KindMain, ForkSource: srcID, ForkBoundary: boundary,
		Coverage: src.Coverage, Metrics: Metrics{Context: src.Metrics.Context},
	}
	var covers int64
	if n := len(revs); n > 0 {
		fork.Summary = revs[n-1].Summary
		covers = revs[n-1].CoversThrough
	}
	var head, active []Turn
	for _, t := range all {
		switch {
		case t.HID > boundary:
		case t.HID <= covers:
			head = append(head, t)
		default:
			active = append(active, t)
		}
	}
	fork.Turns = active
	log := newSafeLog(active, boundary)
	log.archive, log.sums = head, revs
	fork.Log = log
	tx, err := d.Begin()
	if err != nil {
		return nil, "", err
	}
	pend, err := writeSessionTx(tx, fork)
	if err == nil {
		err = copyVersioned(tx, srcID, fork.ID, boundary, src.Legacy)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		_ = tx.Rollback()
		return nil, "", err
	}
	log.ClearPending(len(pend.Archive), len(pend.Summaries))
	warning := ""
	if src.Legacy {
		warning = LegacyForkWarning
	}
	return fork, warning, nil
}

// summaryRevs are the summary revisions of a session valid at boundary: those
// whose coverage ends at or before it, oldest first.
func summaryRevs(d *sql.DB, id string, boundary int64) ([]SummaryRev, error) {
	rows, err := d.Query("SELECT rev,summary,covers_through,created_at FROM summaries WHERE session_id = ? AND covers_through <= ? ORDER BY covers_through, rev", id, boundary)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SummaryRev
	for rows.Next() {
		var r SummaryRev
		var when string
		if err := rows.Scan(&r.Rev, &r.Summary, &r.CoversThrough, &when); err != nil {
			return nil, err
		}
		r.Time = parseTS(when)
		out = append(out, r)
	}
	return out, rows.Err()
}

// copyVersioned writes the versioned state of src at boundary into dst, with a
// journal baseline at the same boundary. A legacy source has no journal worth
// reconstructing, so nothing is copied.
func copyVersioned(tx *sql.Tx, src, dst string, boundary int64, legacy bool) error {
	if legacy {
		return nil
	}
	var ver int64
	for _, ns := range []string{NSData} {
		entries, err := entriesAt(tx, src, ns, boundary)
		if err != nil {
			return err
		}
		for _, e := range entries {
			ver++
			if _, err := tx.Exec("INSERT INTO kv(session_id,ns,key,value_json) VALUES(?,?,?,?)", dst, ns, e.Key, e.JSON); err != nil {
				return err
			}
			if _, err := tx.Exec("INSERT INTO journal(session_id,ver,ns,key,value_json,deleted,boundary,time) VALUES(?,?,?,?,?,0,?,?)",
				dst, ver, ns, e.Key, e.JSON, boundary, ts(time.Now())); err != nil {
				return err
			}
		}
	}
	return nil
}
