package agent

import (
	"database/sql"
	"encoding/json"
)

func writeRequestConfigs(tx *sql.Tx, session string, configs map[string]RequestConfig) error {
	for id, c := range configs {
		raw, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO request_configs(session_id,id,data) VALUES(?,?,?)", session, id, string(raw)); err != nil {
			return err
		}
	}
	return nil
}

func readRequestConfigs(db *sql.DB, session string) (map[string]RequestConfig, error) {
	rows, err := db.Query("SELECT id,data FROM request_configs WHERE session_id = ?", session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]RequestConfig{}
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var c RequestConfig
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, err
		}
		out[id] = c
	}
	return out, rows.Err()
}
