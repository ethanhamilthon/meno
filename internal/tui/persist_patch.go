package tui

import (
	"encoding/json"

	"meno/internal/config"
)

// settingsDelta lists the settings fields that differ between base and next,
// with the values to write and the values the daemon was last seen to hold.
// Only these fields travel, so an edit elsewhere to any other field survives.
func settingsDelta(base, next config.Settings) (fields, expect map[string]any) {
	b, n := toMap(base), toMap(next)
	fields, expect = map[string]any{}, map[string]any{}
	for k, v := range n {
		if string(mustJSON(v)) != string(mustJSON(b[k])) {
			fields[k], expect[k] = v, b[k]
		}
	}
	return fields, expect
}

func toMap(s config.Settings) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(mustJSON(s), &m)
	return m
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
