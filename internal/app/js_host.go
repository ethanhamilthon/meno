package app

import (
	"encoding/json"

	"meno/internal/agent"
)

// toAny turns any JSON-encodable value into plain maps and slices, so the VM
// shows the json field names instead of Go struct fields.
func toAny(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	err = json.Unmarshal(b, &out)
	return out, err
}

// sessionHost implements jsvm.SessionHost for one session. It runs on the
// agent's goroutine, so anything that touches UI state goes through onUI.
type sessionHost struct {
	a     *App
	sid   string
	sess  *agent.Session // set for a subagent, whose session is not in a.rts
	child bool
}
