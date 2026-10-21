package engine

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"meno/internal/agent"
	"meno/internal/proto"
)

// exportResource uses the existing commit boundary and saved event projection.
// Provider deltas after this boundary are intentionally outside the transcript.
func (e *Engine) exportResource(ctx context.Context, id string) (proto.ResourceResponse, error) {
	out, err := e.exportSnapshot(ctx, id)
	if err != nil {
		return proto.ResourceResponse{}, err
	}
	// exportSnapshot has returned and released commit and mu before rendering.
	b, err := json.Marshal(proto.ExportResponse{Markdown: out.Markdown()})
	return proto.ResourceResponse{Data: b}, err
}

func (e *Engine) exportSnapshot(ctx context.Context, id string) (agent.ExportSnapshot, error) {
	s, err := e.load(ctx, id)
	if err != nil {
		return agent.ExportSnapshot{}, notFound(err)
	}
	s.commit.Lock()
	defer s.commit.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := agent.LoadExport(e.home, id)
	if os.IsNotExist(err) && !s.persisted {
		out = agent.ExportSnapshot{ID: id, Title: s.session.DisplayTitle(), Cwd: s.cwd,
			Provider: s.session.Provider, Model: s.session.Model, Effort: s.session.Effort, Cutoff: time.Now()}
		err = nil
	}
	if err != nil {
		return agent.ExportSnapshot{}, err
	}
	out.Running = s.running || s.compacting
	if out.Running {
		out.Warnings = append(out.Warnings, "Active work is incomplete. This export ends at the last persisted event; uncommitted streaming output is not included.")
	} else {
		for i := range out.Requests {
			if out.Requests[i].Outcome == "pending" {
				out.Requests[i].Outcome = "interrupted: final outcome unavailable"
			}
		}
	}
	return out, nil
}
