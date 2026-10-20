package engine

import (
	"context"
	"encoding/json"
	"meno/internal/form"
	"meno/internal/proto"
	"strings"
)

// AnswerAsk delivers one validated response to the paused model call.
func (e *Engine) AnswerAsk(ctx context.Context, id, askID string, q proto.AskAnswerRequest) error {
	s, err := e.load(ctx, id)
	if err != nil {
		return notFound(err)
	}
	s.mu.Lock()
	a := s.ask
	if a == nil || a.id != askID {
		s.mu.Unlock()
		return api(proto.CodeNotFound, "ask is not open")
	}
	s.mu.Unlock()
	if q.Cancel {
		select {
		case a.answer <- askReply{err: form.ErrCancelled}:
			return nil
		default:
			return api(proto.CodeConflict, "ask has already been answered")
		}
	}
	answers := make([]form.Answer, 0, len(a.form.Questions))
	for _, question := range a.form.Questions {
		raw, ok := q.Answers[question.ID]
		if !ok && question.Type != form.TypeText {
			return api(proto.CodeBadRequest, "missing answer for "+question.ID)
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
						return api(proto.CodeBadRequest, "invalid answer for "+question.ID)
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
				return api(proto.CodeBadRequest, "unknown option "+v+" for "+question.ID)
			}
		}
		if question.Type == form.TypeSingle && len(ans.Selected) > 1 {
			return api(proto.CodeBadRequest, "single-choice answer has multiple options")
		}
		if question.Type == form.TypeText && strings.TrimSpace(ans.Text) == "" {
			return api(proto.CodeBadRequest, "answer text is required for "+question.ID)
		}
		answers = append(answers, ans)
	}
	select {
	case a.answer <- askReply{answers: answers}:
		return nil
	default:
		return api(proto.CodeConflict, "ask has already been answered")
	}
}
