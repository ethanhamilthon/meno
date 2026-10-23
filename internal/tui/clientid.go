package tui

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// nowFn is replaced in tests.
var nowFn = time.Now

// newClientID makes the id a client gives to a request, so a retry is safe.
func newClientID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// newInstanceID is a random identity for one running client instance (one
// terminal window). It is not derived from the process ID, which a restart can
// reuse, and it is distinct from the per-request IDs of newClientID.
func newInstanceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// base64Of encodes image bytes for the wire; nil gives "".
func base64Of(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}
