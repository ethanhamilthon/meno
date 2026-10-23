package acp

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// diagnostics is deliberately metadata-only, even when explicitly enabled.
// Unknown method names are not echoed: arbitrary method names can contain text.
type diagnostics struct {
	mu  sync.Mutex
	out io.Writer
}

func (d *diagnostics) record(method string, code int, start time.Time) {
	if d == nil || d.out == nil {
		return
	}
	switch method {
	case "initialize", "session/new", "session/load", "session/list", "session/prompt", "session/cancel", "session/set_config_option":
	default:
		method = "unknown"
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_ = json.NewEncoder(d.out).Encode(struct {
		Method   string `json:"method"`
		Code     int    `json:"code"`
		Duration int64  `json:"duration_ms"`
	}{method, code, time.Since(start).Milliseconds()})
}
