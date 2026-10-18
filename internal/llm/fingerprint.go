package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Fingerprint describes a request without containing any of its text: lengths
// and hashes only. Comparing the fingerprints of two requests shows how much of
// their prefix is byte-identical, which is what a provider's prefix cache needs.
type Fingerprint struct {
	Model      string
	Effort     string
	SystemLen  int
	SystemHash string
	Tools      bool
	Messages   int
	// Lens is the serialized size of each message; Prefix[i] is a hash of the
	// system prompt and messages 0..i, so equal Prefix[i] means an identical prefix.
	Lens   []int
	Prefix []string
}

func short(sum [32]byte) string { return hex.EncodeToString(sum[:4]) }

// FingerprintOf computes the fingerprint of a request.
func FingerprintOf(req Request) Fingerprint {
	f := Fingerprint{Model: req.Model, Effort: req.Effort, SystemLen: len(req.System), Tools: req.Tools, Messages: len(req.Msgs)}
	sys := sha256.Sum256([]byte(req.System))
	f.SystemHash = short(sys)
	h := sha256.New()
	h.Write(sys[:])
	for _, m := range req.Msgs {
		b, _ := json.Marshal(m)
		f.Lens = append(f.Lens, len(b))
		h.Write(b)
		var sum [32]byte
		copy(sum[:], h.Sum(nil))
		f.Prefix = append(f.Prefix, short(sum))
	}
	return f
}

// SharedPrefix is how many leading messages the two requests have in common,
// or -1 when the system prompt, model or tool setting differs (then nothing is
// shared).
func (f Fingerprint) SharedPrefix(prev Fingerprint) int {
	if f.SystemHash != prev.SystemHash || f.Model != prev.Model || f.Tools != prev.Tools {
		return -1
	}
	n := 0
	for n < len(f.Prefix) && n < len(prev.Prefix) && f.Prefix[n] == prev.Prefix[n] {
		n++
	}
	return n
}
