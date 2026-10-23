package engine

import (
	"context"
	"encoding/json"
	"errors"

	"meno/internal/api"
	"meno/internal/form"
)

// askResource opens a form for the human, pauses the timer of the bash call
// that asked, blocks until the form is answered, cancelled or times out, and
// returns the answers.
func (e *Engine) askResource(ctx context.Context, s *runState, call string, data map[string]any) (any, error) {
	if s.session.NoAsk {
		return nil, dapi(api.CodeForbidden, NoAskMessage)
	}
	spec, _ := data["form"].(map[string]any)
	if spec == nil {
		return nil, dapi(api.CodeBadRequest, "ask needs a form object")
	}
	if t, ok := spec["timeout"].(float64); ok {
		spec["timeout"] = int64(t)
	}
	f, err := form.Parse(spec)
	if err != nil {
		return nil, dapi(api.CodeBadRequest, err.Error())
	}
	answers, err := asker{e, s}.Ask(ctx, f)
	switch {
	case errors.Is(err, form.ErrCancelled):
		return nil, dapi(api.CodeConflict, "cancelled")
	case errors.Is(err, form.ErrTimeout):
		return nil, dapi(api.CodeConflict, "timeout")
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

// NoAskMessage is what the model reads when it calls `meno ask` in a session
// that was created with --no-ask.
const NoAskMessage = "asking the user with meno ask is turned off for this session (--no-ask). " +
	"Do not use meno ask: write your question directly in the chat and wait for the user's reply."
