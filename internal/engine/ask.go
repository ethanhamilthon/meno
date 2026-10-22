package engine

import (
	"context"
	"encoding/json"
	"meno/internal/api"
	"meno/internal/form"
	"strings"
)

// AnswerAsk delivers one validated response to the paused model call.
func (e *Engine) AnswerAsk(ctx context.Context, id, askID string, q api.AskAnswerRequest) error {
	s, err := e.load(ctx, id)
	if err != nil {
		return notFound(err)
	}
	s.mu.Lock()
	a := s.ask
	if a == nil || a.id != askID {
		s.mu.Unlock()
		return dapi(api.CodeNotFound, "ask is not open")
	}
	s.mu.Unlock()
	if q.Cancel {
		if !a.claim() {
			return dapi(api.CodeConflict, "ask has already been answered")
		}
		a.answer <- askReply{err: form.ErrCancelled}
		return nil
	}
	answers := make([]form.Answer, 0, len(a.form.Questions))
	for _, question := range a.form.Questions {
		raw, ok := q.Answers[question.ID]
		if !ok && question.Type != form.TypeText {
			return dapi(api.CodeBadRequest, "missing answer for "+question.ID)
		}
		ans := form.Answer{ID: question.ID, Selected: []string{}}
		if len(raw) > 0 {
			var text string
			if json.Unmarshal(raw, &text) == nil {
				ans.Text = text
			} else {
				var selected []string
				if json.Unmarshal(raw, &selected) == nil {
					ans.Selected = selected
				} else {
					var obj struct {
						Text     string   `json:"text"`
						Selected []string `json:"selected"`
					}
					if err = json.Unmarshal(raw, &obj); err != nil {
						return dapi(api.CodeBadRequest, "invalid answer for "+question.ID)
					}
					ans.Text, ans.Selected = obj.Text, obj.Selected
				}
			}
		}
		valid := map[string]bool{}
		for _, o := range question.Options {
			valid[o.ID] = true
		}
		for _, v := range ans.Selected {
			if !valid[v] {
				return dapi(api.CodeBadRequest, "unknown option "+v+" for "+question.ID)
			}
		}
		if question.Type == form.TypeSingle && len(ans.Selected) > 1 {
			return dapi(api.CodeBadRequest, "single-choice answer has multiple options")
		}
		if question.Type == form.TypeText && strings.TrimSpace(ans.Text) == "" {
			return dapi(api.CodeBadRequest, "answer text is required for "+question.ID)
		}
		answers = append(answers, ans)
	}
	// Only a valid answer may take the question: an invalid one returned above
	// leaves it open for the next client.
	if !a.claim() {
		return dapi(api.CodeConflict, "ask has already been answered")
	}
	a.answer <- askReply{answers: answers}
	return nil
}
