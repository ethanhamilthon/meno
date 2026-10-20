package engine

import (
	"context"
	"encoding/json"
	"errors"

	"meno/internal/form"
	"meno/internal/proto"
)

// askResource opens a form for the human, pauses the timer of the bash call
// that asked, blocks until the form is answered, cancelled or times out, and
// returns the answers.
func (e *Engine) askResource(ctx context.Context, s *runState, call string, data map[string]any) (any, error) {
	spec, _ := data["form"].(map[string]any)
	if spec == nil {
		return nil, api(proto.CodeBadRequest, "ask needs a form object")
	}
	if t, ok := spec["timeout"].(float64); ok {
		spec["timeout"] = int64(t)
	}
	f, err := form.Parse(spec)
	if err != nil {
		return nil, api(proto.CodeBadRequest, err.Error())
	}
	s.pauseCall(call, true)
	defer s.pauseCall(call, false)
	answers, err := asker{e, s}.Ask(ctx, f)
	switch {
	case errors.Is(err, form.ErrCancelled):
		return nil, api(proto.CodeConflict, "cancelled")
	case errors.Is(err, form.ErrTimeout):
		return nil, api(proto.CodeConflict, "timeout")
	case err != nil:
		return nil, err
	}
	out := make([]map[string]any, 0, len(answers))
	for _, a := range answers {
		sel := a.Selected
		if sel == nil {
			sel = []string{}
		}
		out = append(out, map[string]any{"id": a.ID, "selected": sel, "text": a.Text})
	}
	b, _ := json.Marshal(out)
	return map[string]json.RawMessage{"answers": b}, nil
}
