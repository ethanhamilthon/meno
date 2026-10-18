package jsvm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dop251/goja"
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

// FormOption is a choice the agent supplies.
type FormOption struct{ ID, Label string }

// FormQuestion is one required question. Custom text is always allowed.
type FormQuestion struct {
	ID, Text, Type string
	Options        []FormOption
	Recommendation string // shown as context, never selected for the user
}

// Form is what askUser puts to the human: all questions at once.
type Form struct {
	Questions []FormQuestion
	Timeout   time.Duration // 0: wait until answered, cancelled or the program exits
}

// FormAnswer answers one question. Selected holds option IDs in the order the
// options were given; Text is the custom (or free) text, "" when none.
type FormAnswer struct {
	ID       string
	Selected []string
	Text     string
}

// Asker shows a Form and returns one answer per question in input order. It
// returns ErrCancelled when the user cancels and must return when ctx ends.
type Asker interface {
	Ask(ctx context.Context, f Form) ([]FormAnswer, error)
}

// Errors an Asker returns; askUser turns them into AskCancelled and AskTimeout.
var (
	ErrCancelled = errors.New("the user cancelled the questions")
	ErrTimeout   = errors.New("the user did not answer in time")
)

// SetAsker enables askUser.
func (v *VM) SetAsker(a Asker) { v.asker.Store(&a) }

func (v *VM) askerOrNil() Asker {
	if p := v.asker.Load(); p != nil {
		return *p
	}
	return nil
}

func str(m map[string]any, k string) (string, bool) {
	s, ok := m[k].(string)
	return s, ok
}

// ParseForm validates the argument of askUser before any UI is shown.
func ParseForm(spec map[string]any) (Form, error) {
	var f Form
	for k := range spec {
		if k != "questions" && k != "timeout" {
			return f, fmt.Errorf("askUser: unknown option %q (allowed: questions, timeout)", k)
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
			return f, errors.New("askUser: timeout must be a number of seconds")
		}
		if math.IsNaN(secs) || math.IsInf(secs, 0) || secs < 0 {
			return f, errors.New("askUser: timeout must be a finite number of seconds, 0 or more")
		}
		f.Timeout = time.Duration(secs * float64(time.Second))
	}
	qs, ok := spec["questions"].([]any)
	if !ok || len(qs) == 0 {
		return f, errors.New("askUser: pass {questions: [{id, text, type, options}], timeout?}; at least one question is required")
	}
	if len(qs) > MaxQuestions {
		return f, fmt.Errorf("askUser: at most %d questions", MaxQuestions)
	}
	seen := map[string]bool{}
	for i, raw := range qs {
		m, ok := raw.(map[string]any)
		if !ok {
			return f, fmt.Errorf("askUser: question %d must be an object", i)
		}
		for k := range m {
			switch k {
			case "id", "text", "type", "options", "recommendation":
			default:
				return f, fmt.Errorf("askUser: question %d: unknown field %q", i, k)
			}
		}
		q := FormQuestion{}
		q.ID, ok = str(m, "id")
		if !ok || q.ID == "" {
			return f, fmt.Errorf("askUser: question %d needs a non-empty string id", i)
		}
		if seen[q.ID] {
			return f, fmt.Errorf("askUser: duplicate question id %q", q.ID)
		}
		seen[q.ID] = true
		if q.Text, ok = str(m, "text"); !ok || q.Text == "" {
			return f, fmt.Errorf("askUser: question %q needs non-empty text", q.ID)
		}
		if q.Type, ok = str(m, "type"); !ok || (q.Type != TypeSingle && q.Type != TypeMulti && q.Type != TypeText) {
			return f, fmt.Errorf("askUser: question %q: type must be single, multi or text", q.ID)
		}
		if r, has := m["recommendation"]; has && r != nil {
			if q.Recommendation, ok = r.(string); !ok {
				return f, fmt.Errorf("askUser: question %q: recommendation must be a string", q.ID)
			}
		}
		opts, hasOpts := m["options"].([]any)
		if raw, has := m["options"]; has && raw != nil && !hasOpts {
			return f, fmt.Errorf("askUser: question %q: options must be an array", q.ID)
		}
		if q.Type == TypeText {
			if len(opts) != 0 {
				return f, fmt.Errorf("askUser: text question %q takes no options", q.ID)
			}
		} else if len(opts) == 0 {
			return f, fmt.Errorf("askUser: question %q needs options (or use type \"text\")", q.ID)
		}
		if len(opts) > MaxOptions {
			return f, fmt.Errorf("askUser: question %q has more than %d options", q.ID, MaxOptions)
		}
		ids := map[string]bool{}
		for j, o := range opts {
			om, ok := o.(map[string]any)
			if !ok {
				return f, fmt.Errorf("askUser: question %q option %d must be {id, label}", q.ID, j)
			}
			opt := FormOption{}
			if opt.ID, ok = str(om, "id"); !ok || opt.ID == "" {
				return f, fmt.Errorf("askUser: question %q option %d needs a non-empty id", q.ID, j)
			}
			if opt.Label, ok = str(om, "label"); !ok || opt.Label == "" {
				return f, fmt.Errorf("askUser: question %q option %q needs a non-empty label", q.ID, opt.ID)
			}
			if ids[opt.ID] {
				return f, fmt.Errorf("askUser: question %q has two options with id %q", q.ID, opt.ID)
			}
			ids[opt.ID] = true
			q.Options = append(q.Options, opt)
		}
		f.Questions = append(f.Questions, q)
	}
	return f, nil
}

func (v *VM) bindAsk() {
	rt := v.rt
	_ = rt.Set("__ask", func(call goja.FunctionCall) goja.Value {
		spec, ok := goja.Undefined().Export(), false
		if len(call.Arguments) > 0 {
			spec = call.Arguments[0].Export()
		}
		m, ok := spec.(map[string]any)
		if !ok {
			panic(rt.NewGoError(errors.New("askUser: pass {questions: [{id, text, type, options}], timeout?}")))
		}
		form, err := ParseForm(m)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		asker := v.askerOrNil()
		if asker == nil {
			panic(rt.NewGoError(errors.New("askUser: interactive input is not available here")))
		}
		parent := v.ctx
		if parent == nil {
			parent = context.Background()
		}
		// The time a human takes to answer is not the code's running time.
		resume := v.pauseTimeout()
		defer resume()
		ctx, cancel := parent, context.CancelFunc(func() {})
		if form.Timeout > 0 {
			ctx, cancel = context.WithTimeout(parent, form.Timeout)
		}
		defer cancel()
		answers, err := asker.Ask(ctx, form)
		switch {
		case err == nil:
		case errors.Is(err, ErrCancelled):
			return rt.ToValue(map[string]any{"status": "cancelled"})
		case parent.Err() == nil && (errors.Is(err, ErrTimeout) || errors.Is(ctx.Err(), context.DeadlineExceeded)):
			return rt.ToValue(map[string]any{"status": "timeout", "seconds": form.Timeout.Seconds()})
		default:
			panic(rt.NewGoError(err)) // parent cancellation and failures propagate
		}
		// Objects are built key by key so the order is always id, selected, text.
		out := make([]any, len(answers))
		for i, a := range answers {
			sel := make([]any, len(a.Selected))
			for j, s := range a.Selected {
				sel[j] = s
			}
			o := rt.NewObject()
			_ = o.Set("id", a.ID)
			_ = o.Set("selected", rt.NewArray(sel...))
			_ = o.Set("text", a.Text)
			out[i] = o
		}
		res := rt.NewObject()
		_ = res.Set("status", "ok")
		_ = res.Set("answers", rt.NewArray(out...))
		return res
	})
}
