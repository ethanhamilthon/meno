package jsvm

import (
	"errors"

	"github.com/dop251/goja"
)

// SessionHost is what the session and sessions globals call into. Options and
// results are plain JSON-compatible values. The host owns validation of the
// session (names, ranges); the VM only converts arguments and errors.
type SessionHost interface {
	Rename(name string) (string, error)
	Models(opts map[string]any) (any, error)
	// Compact schedules replacing old context with summary, keeping at least
	// keep recent messages. It takes effect after the current tool batch.
	Compact(summary string, keep int) error
	List(opts map[string]any) (any, error)
	Search(query string, opts map[string]any) (any, error)
	Read(id string, opts map[string]any) (any, error)
}

// SetSessionHost enables session.* and sessions.*.
func (v *VM) SetSessionHost(h SessionHost) { v.sessionHost.Store(&h) }

func (v *VM) sessions() (SessionHost, error) {
	if p := v.sessionHost.Load(); p != nil {
		return *p, nil
	}
	return nil, errors.New("sessions are not available here")
}

// optsArg reads an optional options object from args[i].
func optsArg(rt *goja.Runtime, args []goja.Value, i int) map[string]any {
	if len(args) <= i || goja.IsUndefined(args[i]) || goja.IsNull(args[i]) {
		return map[string]any{}
	}
	m, ok := args[i].Export().(map[string]any)
	if !ok {
		panic(rt.NewTypeError("options must be an object"))
	}
	return m
}

func strArg(rt *goja.Runtime, args []goja.Value, i int, name string) string {
	if len(args) <= i {
		panic(rt.NewTypeError("%s is required", name))
	}
	s, ok := args[i].Export().(string)
	if !ok {
		panic(rt.NewTypeError("%s must be a string", name))
	}
	return s
}

func (v *VM) bindSession() {
	rt := v.rt
	host := func() SessionHost {
		h, err := v.sessions()
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return h
	}
	check := func(res any, err error) goja.Value {
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(res)
	}
	session := rt.NewObject()
	_ = session.Set("rename", func(c goja.FunctionCall) goja.Value {
		name := strArg(rt, c.Arguments, 0, "name")
		return check(host().Rename(name))
	})
	_ = session.Set("models", func(c goja.FunctionCall) goja.Value {
		return check(host().Models(optsArg(rt, c.Arguments, 0)))
	})
	_ = session.Set("compact", func(c goja.FunctionCall) goja.Value {
		summary := strArg(rt, c.Arguments, 0, "summary")
		if len(c.Arguments) < 2 {
			panic(rt.NewTypeError("saveLastNMessages is required"))
		}
		n, ok := c.Arguments[1].Export().(int64)
		if !ok || n < 0 {
			panic(rt.NewTypeError("saveLastNMessages must be a non-negative integer"))
		}
		return check(map[string]any{"scheduled": true}, host().Compact(summary, int(n)))
	})
	_ = rt.Set("session", session)
	sessions := rt.NewObject()
	_ = sessions.Set("list", func(c goja.FunctionCall) goja.Value {
		return check(host().List(optsArg(rt, c.Arguments, 0)))
	})
	_ = sessions.Set("search", func(c goja.FunctionCall) goja.Value {
		return check(host().Search(strArg(rt, c.Arguments, 0, "query"), optsArg(rt, c.Arguments, 1)))
	})
	_ = sessions.Set("read", func(c goja.FunctionCall) goja.Value {
		return check(host().Read(strArg(rt, c.Arguments, 0, "id"), optsArg(rt, c.Arguments, 1)))
	})
	_ = rt.Set("sessions", sessions)
}
