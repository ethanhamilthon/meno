// Content: is the self-contained form of a quotation from the
// conversation. A reference carries the quoted text itself plus where it came
// from, so it survives queueing, retries, compaction and restarts without any
// pointer into history. Quoted text is the user's data, never an instruction.
package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxQuoteBytes bounds one quoted segment. A longer quote is an error: it is
// never clipped silently.
const MaxQuoteBytes = 16 << 10

// Part names what was quoted from a turn.
const (
	PartAssistant = "assistant"
	PartCode      = "code"
	PartOutput    = "output"
	PartError     = "error"
)

// QuoteSegment is one contiguous quotation and its source.
type QuoteSegment struct {
	SessionID string `json:"session"`
	TurnID    string `json:"turn"`
	CallID    string `json:"call,omitempty"`
	Part      string `json:"part"`
	Quote     string `json:"quote"`
}

// Reference is one or more quotations attached to a single comment.
type Reference struct {
	Segments []QuoteSegment `json:"segments"`
}

// Encode returns the deterministic JSON text of r: the same reference always
// gives the same bytes, so equal selections compare equal.
func EncodeQuote(r Reference) (string, error) {
	if err := validate(r); err != nil {
		return "", err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Decode parses text made by Encode. Unknown fields and trailing data are
// errors so a malformed or forged payload is rejected, not guessed at.
func DecodeQuote(s string) (Reference, error) {
	var r Reference
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return Reference{}, fmt.Errorf("comment reference: %w", err)
	}
	if dec.More() {
		return Reference{}, errors.New("comment reference: trailing data")
	}
	if err := validate(r); err != nil {
		return Reference{}, err
	}
	return r, nil
}

func validate(r Reference) error {
	if len(r.Segments) == 0 {
		return errors.New("comment reference has no segments")
	}
	for i, s := range r.Segments {
		switch s.Part {
		case PartAssistant, PartCode, PartOutput, PartError:
		default:
			return fmt.Errorf("segment %d: unknown part %q", i+1, s.Part)
		}
		if s.SessionID == "" || s.TurnID == "" {
			return fmt.Errorf("segment %d: session and turn are required", i+1)
		}
		if strings.TrimSpace(s.Quote) == "" {
			return fmt.Errorf("segment %d: the quote is empty", i+1)
		}
		if len(s.Quote) > MaxQuoteBytes {
			return fmt.Errorf("segment %d: the quote is %d bytes, the limit is %d", i+1, len(s.Quote), MaxQuoteBytes)
		}
		if !utf8.ValidString(s.Quote) {
			return fmt.Errorf("segment %d: the quote is not valid UTF-8", i+1)
		}
	}
	return nil
}

// Label is the short text shown in the composer and timeline: the first three
// words of the first quote.
func (r Reference) Label() string {
	if len(r.Segments) == 0 {
		return "[Comment]"
	}
	words := strings.Fields(r.Segments[0].Quote)
	if len(words) == 0 {
		return "[Comment]"
	}
	more := ""
	if len(words) > 3 {
		words, more = words[:3], "..."
	}
	return "[Comment - " + strings.Join(words, " ") + more + "]"
}
