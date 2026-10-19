package jsvm

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// TaskCall carries what a tasks.* call captured from the running call: the
// working directory and the environment as they are now.
type TaskCall struct {
	Ctx  context.Context
	Cwd  string
	Env  []string // KEY=VALUE, the call's local copy
	Opts map[string]any
}

// TasksHost is the background process supervisor behind the tasks global. It
// exists only for the main agent.
type TasksHost interface {
	Start(c TaskCall, command string) (any, error)
	List(c TaskCall) (any, error)
	Status(c TaskCall, id string) (any, error)
	Logs(c TaskCall, id string) (any, error)
	Wait(c TaskCall, id string, timeout time.Duration) (any, error)
	Cancel(c TaskCall, id string) (any, error)
}

// SetTasks enables the tasks global.
func (v *VM) SetTasks(h TasksHost) { v.tasksHost.Store(&h) }

// stringMap reads {K: "v"} options.
func stringMap(rt *goja.Runtime, v any, what string) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		panic(rt.NewTypeError("%s must be an object of strings", what))
	}
	out := map[string]string{}
	for k, val := range m {
		s, ok := val.(string)
		if !ok {
			panic(rt.NewTypeError("%s.%s must be a string", what, k))
		}
		out[k] = s
	}
	return out
}

func (v *VM) bindTasks() {
	rt := v.rt
	p := v.tasksHost.Load()
	if p == nil {
		return
	}
	h := *p
	call := func(opts map[string]any) TaskCall {
		return TaskCall{Ctx: v.runCtx(), Cwd: v.Root(), Env: v.shellEnv(), Opts: opts}
	}
	check := func(res any, err error) goja.Value {
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(res)
	}
	obj := rt.NewObject()
	_ = obj.Set("start", func(c goja.FunctionCall) goja.Value {
		if err := v.writable("tasks.start"); err != nil {
			panic(rt.NewGoError(err))
		}
		cmd := strArg(rt, c.Arguments, 0, "command")
		o := optsArg(rt, c.Arguments, 1)
		checkKeys(rt, "tasks.start", o, "cwd", "env", "name", "timeout", "notify")
		tc := call(o)
		if d, ok := o["cwd"].(string); ok && d != "" {
			if !filepath.IsAbs(d) {
				d = filepath.Join(tc.Cwd, d)
			}
			tc.Cwd = d
		} else if _, set := o["cwd"]; set {
			panic(rt.NewTypeError("cwd must be a non-empty string"))
		}
		if e, ok := o["env"]; ok {
			over := stringMap(rt, e, "env")
			// The environment is a replace list, not a concatenation: a
			// duplicate name would otherwise leave the old value in front.
			env := map[string]string{}
			for _, kv := range tc.Env {
				if k, v, ok := strings.Cut(kv, "="); ok {
					env[k] = v
				}
			}
			for k, v := range over {
				env[k] = v
			}
			keys := make([]string, 0, len(env))
			for k := range env {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			tc.Env = tc.Env[:0]
			for _, k := range keys {
				tc.Env = append(tc.Env, k+"="+env[k])
			}
		}
		if t, ok := o["timeout"]; ok {
			n, isNum := t.(int64)
			f, isF := t.(float64)
			if !isNum && !isF || (isNum && n < 0) || (isF && (f < 0 || f != f)) {
				panic(rt.NewTypeError("timeout must be a finite number of seconds, 0 or more"))
			}
		}
		if n, ok := o["notify"]; ok {
			if _, isBool := n.(bool); !isBool {
				panic(rt.NewTypeError("notify must be true or false"))
			}
		}
		return check(h.Start(tc, cmd))
	})
	_ = obj.Set("list", func(c goja.FunctionCall) goja.Value {
		o := optsArg(rt, c.Arguments, 0)
		checkKeys(rt, "tasks.list", o, "includeFinished")
		return check(h.List(call(o)))
	})
	_ = obj.Set("status", func(c goja.FunctionCall) goja.Value {
		return check(h.Status(call(nil), strArg(rt, c.Arguments, 0, "id")))
	})
	_ = obj.Set("logs", func(c goja.FunctionCall) goja.Value {
		id := strArg(rt, c.Arguments, 0, "id")
		o := optsArg(rt, c.Arguments, 1)
		checkKeys(rt, "tasks.logs", o, "cursor", "limit", "stream")
		return check(h.Logs(call(o), id))
	})
	_ = obj.Set("wait", func(c goja.FunctionCall) goja.Value {
		id := strArg(rt, c.Arguments, 0, "id")
		o := optsArg(rt, c.Arguments, 1)
		checkKeys(rt, "tasks.wait", o, "timeout")
		var timeout time.Duration
		if t, ok := o["timeout"]; ok {
			switch n := t.(type) {
			case int64:
				timeout = time.Duration(n) * time.Second
			case float64:
				timeout = time.Duration(n * float64(time.Second))
			default:
				panic(rt.NewTypeError("timeout must be a number of seconds"))
			}
			if timeout < 0 {
				panic(rt.NewTypeError("timeout must not be negative"))
			}
		}
		// A long wait is not the code running: it has its own timeout.
		resume := v.pauseTimeout()
		defer resume()
		return check(h.Wait(call(o), id, timeout))
	})
	_ = obj.Set("cancel", func(c goja.FunctionCall) goja.Value {
		return check(h.Cancel(call(nil), strArg(rt, c.Arguments, 0, "id")))
	})
	_ = rt.Set("tasks", obj)
}

// runCtx is the context of the running call (cancellation of the request).
func (v *VM) runCtx() context.Context {
	if v.ctx == nil {
		return context.Background()
	}
	return v.ctx
}
