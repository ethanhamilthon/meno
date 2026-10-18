package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"meno/internal/llm"
)

// debugPayload returns the opt-in request logger: with MENO_DEBUG_PAYLOADS=1
// every request is appended, in full, to $MENO_HOME/debug/requests.jsonl. The
// provider and its API key are never written. It is off by default because the
// log contains your messages and file contents.
func (a *App) debugPayload() func(llm.Request) {
	if os.Getenv("MENO_DEBUG_PAYLOADS") != "1" {
		return nil
	}
	dir := filepath.Join(a.paths.Home, "debug")
	var mu sync.Mutex
	return func(r llm.Request) {
		mu.Lock()
		defer mu.Unlock()
		_ = os.MkdirAll(dir, 0o700)
		f, err := os.OpenFile(filepath.Join(dir, "requests.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		defer f.Close()
		_ = json.NewEncoder(f).Encode(map[string]any{"time": time.Now().UTC(), "model": r.Model, "effort": r.Effort, "system": r.System, "messages": r.Msgs})
	}
}
