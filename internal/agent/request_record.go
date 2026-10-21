package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"meno/internal/llm"
)

// RequestConfig contains only the effective, credential-free model instructions.
// IDs are content hashes, so repeated prompts do not multiply storage.
type RequestConfig struct {
	ID     string          `json:"id"`
	System string          `json:"system"`
	Tool   json.RawMessage `json:"tool,omitempty"`
}

// RequestRecord describes one provider attempt, not one tool invocation.
// Usage nil means unavailable; a non-nil zero value is reported zero usage.
type RequestRecord struct {
	ID       string     `json:"id"`
	Step     string     `json:"step"`
	Attempt  int        `json:"attempt"`
	Purpose  string     `json:"purpose"`
	Boundary int64      `json:"boundary"`
	Provider string     `json:"provider"`
	Model    string     `json:"model"`
	Effort   string     `json:"effort"`
	Config   string     `json:"config"`
	Started  time.Time  `json:"started"`
	Ended    time.Time  `json:"ended,omitempty"`
	Outcome  string     `json:"outcome"`
	Usage    *llm.Usage `json:"usage,omitempty"`
}

func (s *Session) startRequest(req llm.Request, step string, attempt int, purpose string) string {
	c := RequestConfig{System: req.System, Tool: req.ExportToolDefinition()}
	raw, _ := json.Marshal(c)
	hash := sha256.Sum256(raw)
	c.ID = hex.EncodeToString(hash[:])
	r := &RequestRecord{ID: NewID(), Step: step, Attempt: attempt, Purpose: purpose,
		Boundary: s.ExecBoundary(), Provider: req.Provider.Name, Model: req.Model,
		Effort: req.Effort, Config: c.ID, Started: time.Now(), Outcome: "pending"}
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	if s.requestConfigs == nil {
		s.requestConfigs = map[string]RequestConfig{}
	}
	s.requestConfigs[c.ID] = c
	s.Requests = append(s.Requests, RequestUsage{Provider: r.Provider, Model: r.Model, Time: r.Started, Record: r})
	return r.ID
}

func (s *Session) requestUsage(id string, u llm.Usage) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	for i := range s.Requests {
		r := &s.Requests[i]
		if r.Record != nil && r.Record.ID == id {
			r.Usage = u
			r.Record.Usage = &u
			return
		}
	}
}

func (s *Session) endRequest(id string, err error, cancelled bool) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	for i := range s.Requests {
		r := s.Requests[i].Record
		if r == nil || r.ID != id {
			continue
		}
		r.Ended, r.Outcome = time.Now(), "completed"
		if err != nil {
			r.Outcome = "failed"
		}
		if cancelled {
			r.Outcome = "cancelled"
		}
		return
	}
}

// requestSnapshot deep-copies the mutable accounting under its own lock. This
// lock is never held while invoking event callbacks or provider code.
func (s *Session) requestSnapshot() ([]RequestUsage, map[string]RequestConfig) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	out := append([]RequestUsage(nil), s.Requests...)
	for i := range out {
		if r := out[i].Record; r != nil {
			v := *r
			if r.Usage != nil {
				u := *r.Usage
				v.Usage = &u
			}
			out[i].Record = &v
		}
	}
	configs := make(map[string]RequestConfig, len(s.requestConfigs))
	for id, c := range s.requestConfigs {
		c.Tool = append(json.RawMessage(nil), c.Tool...)
		configs[id] = c
	}
	return out, configs
}

func copyRequestAncestry(dst, src *Session, boundary int64) {
	records, configs := src.requestSnapshot()
	dst.requestConfigs = map[string]RequestConfig{}
	for _, r := range records {
		// Legacy rows have no reliable boundary and cannot be copied honestly.
		if r.Record == nil || r.Record.Boundary > boundary || r.Record.Outcome == "pending" {
			continue
		}
		dst.Requests = append(dst.Requests, r)
		dst.requestConfigs[r.Record.Config] = configs[r.Record.Config]
	}
}
