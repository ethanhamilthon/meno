package jsvm

import (
	"time"

	"github.com/dop251/goja"
)

// AgentsHost is the subagent manager behind the agents global. The main agent
// gets spawn/list/status/send/wait/result/cancel, a child gets only
// report/block; the other methods are not bound for it.
type AgentsHost interface {
	Spawn(c TaskCall, spec map[string]any) (any, error)
	List(c TaskCall) (any, error)
	Status(c TaskCall, id string) (any, error)
	Send(c TaskCall, id, text string) (any, error)
	Wait(c TaskCall, id string, timeout time.Duration) (any, error)
	Result(c TaskCall, id string) (any, error)
	Cancel(c TaskCall, id string) (any, error)
	Report(c TaskCall, text string) error
	Block(c TaskCall, reason string) error
}

// SetAgents enables the agents global; child selects the child API.
func (v *VM) SetAgents(h AgentsHost, child bool) {
	v.agentsHost.Store(&h)
	v.agentsChild.Store(child)
}

func (v *VM) bindAgents() {
	rt := v.rt
	p := v.agentsHost.Load()
	if p == nil {
		return
	}
	h := *p
	call := func(o map[string]any) TaskCall {
		return TaskCall{Ctx: v.runCtx(), Cwd: v.Root(), Env: v.shellEnv(), Opts: o}
	}
	check := func(res any, err error) goja.Value {
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(res)
	}
	obj := rt.NewObject()
	if v.agentsChild.Load() {
		_ = obj.Set("report", func(c goja.FunctionCall) goja.Value {
			text := strArg(rt, c.Arguments, 0, "text")
			if err := h.Report(call(nil), text); err != nil {
				panic(rt.NewGoError(err))
			}
			return goja.Undefined()
		})
		_ = obj.Set("block", func(c goja.FunctionCall) goja.Value {
			reason := strArg(rt, c.Arguments, 0, "reason")
			if err := h.Block(call(nil), reason); err != nil {
				panic(rt.NewGoError(err))
			}
			return goja.Undefined()
		})
		_ = rt.Set("agents", obj)
		return
	}
	_ = obj.Set("spawn", func(c goja.FunctionCall) goja.Value {
		o := optsArg(rt, c.Arguments, 0)
		checkKeys(rt, "agents.spawn", o, "task", "cwd", "provider", "model", "effort")
		return check(h.Spawn(call(o), o))
	})
	_ = obj.Set("list", func(c goja.FunctionCall) goja.Value { return check(h.List(call(nil))) })
	_ = obj.Set("status", func(c goja.FunctionCall) goja.Value {
		return check(h.Status(call(nil), strArg(rt, c.Arguments, 0, "id")))
	})
	_ = obj.Set("send", func(c goja.FunctionCall) goja.Value {
		return check(h.Send(call(nil), strArg(rt, c.Arguments, 0, "id"), strArg(rt, c.Arguments, 1, "text")))
	})
	_ = obj.Set("wait", func(c goja.FunctionCall) goja.Value {
		id := strArg(rt, c.Arguments, 0, "id")
		o := optsArg(rt, c.Arguments, 1)
		checkKeys(rt, "agents.wait", o, "timeout")
		var timeout time.Duration
		switch n := o["timeout"].(type) {
		case nil:
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
		resume := v.pauseTimeout()
		defer resume()
		return check(h.Wait(call(o), id, timeout))
	})
	_ = obj.Set("result", func(c goja.FunctionCall) goja.Value {
		return check(h.Result(call(nil), strArg(rt, c.Arguments, 0, "id")))
	})
	_ = obj.Set("cancel", func(c goja.FunctionCall) goja.Value {
		return check(h.Cancel(call(nil), strArg(rt, c.Arguments, 0, "id")))
	})
	_ = rt.Set("agents", obj)
}
