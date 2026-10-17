package jsvm

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dop251/goja"
)

// Question is what askUser puts to the human.
type Question struct {
	Text         string
	Choices      []string      // empty: free text
	CustomAnswer bool          // with Choices: may the user type something else
	Timeout      time.Duration // 0: wait as long as it takes
}

// Asker shows a Question to the user and returns the answer. It must return
// ErrDeclined when the user refuses, and return when ctx ends.
type Asker interface {
	Ask(ctx context.Context, q Question) (string, error)
}

// ErrDeclined is what an Asker returns when the user refuses to answer.
var ErrDeclined = errors.New("user declined to answer")

// SetAsker enables askUser.
func (v *VM) SetAsker(a Asker) { v.asker.Store(&a) }

func (v *VM) askerOrNil() Asker {
	if p := v.asker.Load(); p != nil {
		return *p
	}
	return nil
}

const maxChoices = 20

// parseQuestion validates the arguments of askUser(question, {choices, customAnswer, timeout}).
func parseQuestion(args []goja.Value) (Question, error) {
	q := Question{CustomAnswer: true}
	if len(args) == 0 || args[0] == nil {
		return q, errors.New("askUser: the question is required")
	}
	text, ok := args[0].Export().(string)
	if !ok || text == "" {
		return q, errors.New("askUser: the question must be a non-empty string")
	}
	q.Text = text
	if len(args) < 2 || goja.IsUndefined(args[1]) || goja.IsNull(args[1]) {
		return q, nil
	}
	opts, ok := args[1].Export().(map[string]any)
	if !ok {
		return q, errors.New("askUser: the second argument must be an object {choices, customAnswer, timeout}")
	}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := opts[k]
		switch k {
		case "choices":
			list, ok := v.([]any)
			if !ok {
				return q, errors.New("askUser: choices must be an array of strings")
			}
			if len(list) > maxChoices {
				return q, fmt.Errorf("askUser: at most %d choices", maxChoices)
			}
			for _, c := range list {
				s, ok := c.(string)
				if !ok || s == "" {
					return q, errors.New("askUser: choices must be non-empty strings")
				}
				q.Choices = append(q.Choices, s)
			}
		case "customAnswer":
			b, ok := v.(bool)
			if !ok {
				return q, errors.New("askUser: customAnswer must be true or false")
			}
			q.CustomAnswer = b
		case "timeout":
			var secs float64
			switch n := v.(type) {
			case int64:
				secs = float64(n)
			case float64:
				secs = n
			default:
				return q, errors.New("askUser: timeout must be a number of seconds")
			}
			if secs < 0 {
				return q, errors.New("askUser: timeout must not be negative")
			}
			q.Timeout = time.Duration(secs * float64(time.Second))
		default:
			return q, fmt.Errorf("askUser: unknown option %q (allowed: choices, customAnswer, timeout)", k)
		}
	}
	if !q.CustomAnswer && len(q.Choices) == 0 {
		return q, errors.New("askUser: customAnswer=false requires choices")
	}
	return q, nil
}

func (v *VM) bindAsk() {
	rt := v.rt
	_ = rt.GlobalObject().DefineDataProperty("askUser", rt.ToValue(func(call goja.FunctionCall) goja.Value {
		q, err := parseQuestion(call.Arguments)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		asker := v.askerOrNil()
		if asker == nil {
			panic(rt.NewGoError(errors.New("askUser: interactive input is not available")))
		}
		parent := v.ctx
		if parent == nil {
			parent = context.Background()
		}
		// The time a human takes to answer is not the code's running time.
		resume := v.pauseTimeout()
		defer resume()
		ctx, cancel := parent, context.CancelFunc(func() {})
		if q.Timeout > 0 {
			ctx, cancel = context.WithTimeout(parent, q.Timeout)
		}
		defer cancel()
		ans, err := asker.Ask(ctx, q)
		if err != nil {
			switch {
			case errors.Is(err, ErrDeclined):
				err = ErrDeclined
			case parent.Err() == nil && errors.Is(ctx.Err(), context.DeadlineExceeded):
				err = fmt.Errorf("user did not answer within %gs", q.Timeout.Seconds())
			}
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(ans)
	}), goja.FLAG_TRUE, goja.FLAG_FALSE, goja.FLAG_FALSE)
}
