package daemonapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"meno/internal/cli"
	"meno/internal/proto"
)

func clientID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("meno-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func decode(raw json.RawMessage, v any) error { return json.Unmarshal(raw, v) }

func toEvent(e proto.Event) cli.Event {
	out := cli.Event{Sequence: e.Seq, Type: e.Type}
	var m map[string]any
	if json.Unmarshal(e.Data, &m) == nil {
		out.Data = m
	}
	return out
}
