package agent

import (
	"sort"
	"time"
)

// ExportSnapshot is an immutable, credential-free export contract. It is built
// from the last persisted event boundary, not a presentation projection.
type ExportSnapshot struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Cwd            string          `json:"cwd"`
	Provider       string          `json:"provider"`
	Model          string          `json:"model"`
	Effort         string          `json:"effort"`
	Cutoff         time.Time       `json:"cutoff"`
	Running        bool            `json:"running"`
	Turns          []Turn          `json:"turns"`
	Configurations []RequestConfig `json:"configurations"`
	Requests       []RequestRecord `json:"requests"`
	Warnings       []string        `json:"warnings,omitempty"`
}

// LoadExport reads saved history including turns archived by compaction. The
// engine serializes this read with session persistence. No current configuration
// is substituted for missing historical request details.
func LoadExport(dir, id string) (ExportSnapshot, error) {
	s, err := LoadSession(dir, id)
	if err != nil {
		return ExportSnapshot{}, err
	}
	turns, err := AllTurns(dir, id)
	if err != nil {
		return ExportSnapshot{}, err
	}
	for i := range turns {
		if turns[i].Thinking == "" && turns[i].Replay != nil {
			turns[i].Thinking = turns[i].Replay.Thought
		}
		// Replay contains opaque provider state, not exportable conversation data.
		turns[i].Replay = nil
	}
	out := ExportSnapshot{ID: s.ID, Title: s.DisplayTitle(), Cwd: s.Cwd,
		Provider: s.Provider, Model: s.Model, Effort: s.Effort, Cutoff: time.Now(), Turns: turns}
	records, configs := s.requestSnapshot()
	for _, r := range records {
		if r.Record != nil {
			out.Requests = append(out.Requests, *r.Record)
		} else {
			out.Requests = append(out.Requests, RequestRecord{Provider: r.Provider, Model: r.Model,
				Started: r.Time, Outcome: "legacy: details unavailable", Usage: &r.Usage})
		}
	}
	for _, c := range configs {
		out.Configurations = append(out.Configurations, c)
	}
	sort.Slice(out.Configurations, func(i, j int) bool { return out.Configurations[i].ID < out.Configurations[j].ID })
	if len(out.Requests) == 0 || s.Legacy || (len(out.Requests) > 0 && (out.Requests[0].Config == "" || out.Requests[0].Boundary > 2)) {
		out.Warnings = append(out.Warnings, "Historical request details are unavailable for some or all of this session; they cannot be reconstructed from current settings.")
	}
	if s.Coverage == CoveragePartial {
		out.Warnings = append(out.Warnings, "Partial history: original turns discarded by an older compaction are unavailable.")
	}
	return out, nil
}
