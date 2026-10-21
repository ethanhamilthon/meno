package app

import (
	"encoding/json"
	"errors"
	"strings"
)

func mapJSON(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(m)
	return string(b)
}
func parseMapJSON(text string) (map[string]string, error) {
	var out map[string]string
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, errors.New("expected a JSON object with string values")
	}
	return out, nil
}

// splitArgs splits a command line into arguments. An empty line gives no
// arguments.
func splitArgs(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	return strings.Fields(text)
}
