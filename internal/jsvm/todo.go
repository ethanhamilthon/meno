package jsvm

import (
	"github.com/dop251/goja"
)

// TodoHost is the plan of the session behind the todo global. The host
// validates; options arrive as plain maps.
type TodoHost interface {
	List(includeArchived bool) (any, error)
	Add(opts map[string]any) (any, error)
	Update(id string, opts map[string]any) (any, error)
	Done(id string, opts map[string]any) (any, error)
	Remove(id string) (bool, error)
	Archive(opts map[string]any) (any, error)
	CriteriaList() (any, error)
	CriteriaAdd(opts map[string]any) (any, error)
	CriteriaUpdate(id string, opts map[string]any) (any, error)
	CriteriaRemove(id string, opts map[string]any) (bool, error)
}

// SetTodo enables the todo global.
func (v *VM) SetTodo(h TodoHost) { v.todoHost.Store(&h) }

func (v *VM) bindTodo() {
	rt := v.rt
	p := v.todoHost.Load()
	if p == nil {
		return // children and tests without a plan have no todo global
	}
	h := *p
	check := func(res any, err error) goja.Value {
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(res)
	}
	obj := rt.NewObject()
	_ = obj.Set("list", func(c goja.FunctionCall) goja.Value {
		o := optsArg(rt, c.Arguments, 0)
		checkKeys(rt, "todo.list", o, "includeArchived")
		inc, _ := o["includeArchived"].(bool)
		return check(h.List(inc))
	})
	_ = obj.Set("add", func(c goja.FunctionCall) goja.Value { return check(h.Add(optsArg(rt, c.Arguments, 0))) })
	_ = obj.Set("update", func(c goja.FunctionCall) goja.Value {
		return check(h.Update(strArg(rt, c.Arguments, 0, "id"), optsArg(rt, c.Arguments, 1)))
	})
	_ = obj.Set("done", func(c goja.FunctionCall) goja.Value {
		return check(h.Done(strArg(rt, c.Arguments, 0, "id"), optsArg(rt, c.Arguments, 1)))
	})
	_ = obj.Set("remove", func(c goja.FunctionCall) goja.Value {
		ok, err := h.Remove(strArg(rt, c.Arguments, 0, "id"))
		return check(ok, err)
	})
	_ = obj.Set("archive", func(c goja.FunctionCall) goja.Value { return check(h.Archive(optsArg(rt, c.Arguments, 0))) })
	crit := rt.NewObject()
	_ = crit.Set("list", func(c goja.FunctionCall) goja.Value { return check(h.CriteriaList()) })
	_ = crit.Set("add", func(c goja.FunctionCall) goja.Value { return check(h.CriteriaAdd(optsArg(rt, c.Arguments, 0))) })
	_ = crit.Set("update", func(c goja.FunctionCall) goja.Value {
		return check(h.CriteriaUpdate(strArg(rt, c.Arguments, 0, "id"), optsArg(rt, c.Arguments, 1)))
	})
	_ = crit.Set("remove", func(c goja.FunctionCall) goja.Value {
		ok, err := h.CriteriaRemove(strArg(rt, c.Arguments, 0, "id"), optsArg(rt, c.Arguments, 1))
		return check(ok, err)
	})
	_ = obj.Set("criteria", crit)
	_ = rt.Set("todo", obj)
}
