package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"meno/internal/api"
)

func clientID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("meno-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func decode(raw json.RawMessage, v any) error { return json.Unmarshal(raw, v) }

func toEvent(e api.Event) Event {
	out := Event{Sequence: e.Seq, Type: e.Type}
	var m map[string]any
	if json.Unmarshal(e.Data, &m) == nil {
		out.Data = m
	}
	return out
}
