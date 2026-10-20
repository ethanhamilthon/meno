// Package form is the shape of the questions an agent puts to the user: a
// form with single-choice, multi-choice and text questions, and its answers.
package form

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Question types.
const (
	TypeSingle = "single"
	TypeMulti  = "multi"
	TypeText   = "text"
)

// Limits of one form.
const (
	MaxQuestions = 20
	MaxOptions   = 20
)

// Option is a choice the agent supplies.
type Option struct{ ID, Label string }

// Question is one required question. Custom text is always allowed.
type Question struct {
	ID, Text, Type string
	Options        []Option
	Recommendation string // shown as context, never selected for the user
}

// Form is what askUser puts to the human: all questions at once.
type Form struct {
	Questions []Question
	Timeout   time.Duration // 0: wait until answered, cancelled or the program exits
}

// Answer answers one question. Selected holds option IDs in the order the
// options were given; Text is the custom (or free) text, "" when none.
type Answer struct {
	ID       string
	Selected []string
	Text     string
}

// Errors an ask ends with.
var (
	ErrCancelled = errors.New("the user cancelled the questions")
	ErrTimeout   = errors.New("the user did not answer in time")
)

func str(m map[string]any, k string) (string, bool) {
	s, ok := m[k].(string)
	return s, ok
}

// Parse validates the argument of askUser before any UI is shown.
func Parse(spec map[string]any) (Form, error) {
	var f Form
	for k := range spec {
		if k != "questions" && k != "timeout" {
			return f, fmt.Errorf("ask: unknown option %q (allowed: questions, timeout)", k)
		}
	}
	if v, ok := spec["timeout"]; ok && v != nil {
		var secs float64
		switch n := v.(type) {
		case int64:
			secs = float64(n)
		case float64:
			secs = n
		default:
			return f, errors.New("ask: timeout must be a number of seconds")
		}
		if math.IsNaN(secs) || math.IsInf(secs, 0) || secs < 0 {
			return f, errors.New("ask: timeout must be a finite number of seconds, 0 or more")
		}
		f.Timeout = time.Duration(secs * float64(time.Second))
	}
	qs, ok := spec["questions"].([]any)
	if !ok || len(qs) == 0 {
		return f, errors.New("ask: pass {questions: [{id, text, type, options}], timeout?}; at least one question is required")
	}
	if len(qs) > MaxQuestions {
		return f, fmt.Errorf("ask: at most %d questions", MaxQuestions)
	}
	seen := map[string]bool{}
	for i, raw := range qs {
		m, ok := raw.(map[string]any)
		if !ok {
			return f, fmt.Errorf("ask: question %d must be an object", i)
		}
		for k := range m {
			switch k {
			case "id", "text", "type", "options", "recommendation":
			default:
				return f, fmt.Errorf("ask: question %d: unknown field %q", i, k)
			}
		}
		q := Question{}
		q.ID, ok = str(m, "id")
		if !ok || q.ID == "" {
			return f, fmt.Errorf("ask: question %d needs a non-empty string id", i)
		}
		if seen[q.ID] {
			return f, fmt.Errorf("ask: duplicate question id %q", q.ID)
		}
		seen[q.ID] = true
		if q.Text, ok = str(m, "text"); !ok || q.Text == "" {
			return f, fmt.Errorf("ask: question %q needs non-empty text", q.ID)
		}
		if q.Type, ok = str(m, "type"); !ok || (q.Type != TypeSingle && q.Type != TypeMulti && q.Type != TypeText) {
			return f, fmt.Errorf("ask: question %q: type must be single, multi or text", q.ID)
		}
		if r, has := m["recommendation"]; has && r != nil {
			if q.Recommendation, ok = r.(string); !ok {
				return f, fmt.Errorf("ask: question %q: recommendation must be a string", q.ID)
			}
		}
		opts, hasOpts := m["options"].([]any)
		if raw, has := m["options"]; has && raw != nil && !hasOpts {
			return f, fmt.Errorf("ask: question %q: options must be an array", q.ID)
		}
		if q.Type == TypeText {
			if len(opts) != 0 {
				return f, fmt.Errorf("ask: text question %q takes no options", q.ID)
			}
		} else if len(opts) == 0 {
			return f, fmt.Errorf("ask: question %q needs options (or use type \"text\")", q.ID)
		}
		if len(opts) > MaxOptions {
			return f, fmt.Errorf("ask: question %q has more than %d options", q.ID, MaxOptions)
		}
		ids := map[string]bool{}
		for j, o := range opts {
			om, ok := o.(map[string]any)
			if !ok {
				return f, fmt.Errorf("ask: question %q option %d must be {id, label}", q.ID, j)
			}
			opt := Option{}
			if opt.ID, ok = str(om, "id"); !ok || opt.ID == "" {
				return f, fmt.Errorf("ask: question %q option %d needs a non-empty id", q.ID, j)
			}
			if opt.Label, ok = str(om, "label"); !ok || opt.Label == "" {
				return f, fmt.Errorf("ask: question %q option %q needs a non-empty label", q.ID, opt.ID)
			}
			if ids[opt.ID] {
				return f, fmt.Errorf("ask: question %q has two options with id %q", q.ID, opt.ID)
			}
			ids[opt.ID] = true
			q.Options = append(q.Options, opt)
		}
		f.Questions = append(f.Questions, q)
	}
	return f, nil
}
