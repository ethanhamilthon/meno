package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"meno/internal/llm"
)

// cacheReport explains, from fingerprints and usage only, how the last request
// related to the one before it. It contains no message text.
func (a *App) cacheReport() {
	fps := a.st.Fps
	if len(fps) == 0 {
		a.notify("cache: no request sent in this session yet")
		return
	}
	last := fps[len(fps)-1]
	head := fmt.Sprintf("last request: %s, system %d chars #%s, %d messages", last.Model, last.SystemLen, last.SystemHash, last.Messages)
	if len(fps) == 1 {
		a.notify(head + " · no earlier request to compare")
		return
	}
	prev := fps[0]
	shared := last.SharedPrefix(prev)
	var cmp string
	switch {
	case shared < 0:
		cmp = "the system prompt, model or tools changed: nothing can be served from the prefix cache"
	case shared == prev.Messages:
		cmp = fmt.Sprintf("the previous request's %d messages are all a prefix of this one", shared)
	default:
		cmp = fmt.Sprintf("prefix shared through message %d of the previous %d (it diverges at message %d)", shared, prev.Messages, shared+1)
	}
	a.notify(head + " · " + cmp + " · usage: last " + cacheText(a.st.LastUsage))
}

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
