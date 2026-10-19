package jsvm

import "github.com/dop251/goja"

// GoalHost receives the verdict of a run in a goal mode.
type GoalHost interface {
	Done(summary string) error
	Blocked(reason string) error
}

// SetGoal enables goal.done and goal.blocked for the following calls; nil
// turns them off.
func (v *VM) SetGoal(h GoalHost) {
	if h == nil {
		v.goalHost.Store(nil)
		return
	}
	v.goalHost.Store(&h)
}

// bindGoal defines the goal global. It is always there, so the runtime does not
// depend on the mode; outside a goal mode its functions say so.
func (v *VM) bindGoal() {
	rt := v.rt
	call := func(name string, f func(h GoalHost, text string) error) func(goja.FunctionCall) goja.Value {
		return func(c goja.FunctionCall) goja.Value {
			p := v.goalHost.Load()
			if p == nil {
				panic(rt.NewTypeError("goal.%s is only available in a #goal run", name))
			}
			if err := f(*p, strArg(rt, c.Arguments, 0, "text")); err != nil {
				panic(rt.NewGoError(err))
			}
			return goja.Undefined()
		}
	}
	obj := rt.NewObject()
	_ = obj.Set("done", call("done", func(h GoalHost, s string) error { return h.Done(s) }))
	_ = obj.Set("blocked", call("blocked", func(h GoalHost, s string) error { return h.Blocked(s) }))
	_ = rt.Set("goal", obj)
}
