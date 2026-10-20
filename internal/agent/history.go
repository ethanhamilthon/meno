package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

func readHistory(d *sql.DB, id string, after int64, limit int) ([]Turn, error) {
	if limit <= 0 {
		limit = 50
	}
	var out []Turn
	rows, err := d.Query("SELECT turn FROM history WHERE session_id = ? AND hid > ? ORDER BY hid LIMIT ?", id, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var t Turn
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return nil, fmt.Errorf("history of %s: %w", id, err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) >= limit {
		return out, nil
	}
	// Archived turns always precede active ones, so the active part continues
	// where the archive ends.
	s, err := readSession(d, id)
	if err != nil {
		return nil, err
	}
	for _, t := range s.Turns {
		if t.HID > after && len(out) < limit && (len(out) == 0 || t.HID > out[len(out)-1].HID) {
			out = append(out, t)
		}
	}
	return out, nil
}
