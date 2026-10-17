package jsvm

import (
	"context"

	"github.com/dop251/goja"
)

// MCPAdmin lets code manage the MCP server configuration. Results are plain
// maps; failures are reported in them ({ok: false, error}) rather than thrown.
type MCPAdmin interface {
	List() []map[string]any
	Add(ctx context.Context, spec map[string]any) map[string]any
	Update(ctx context.Context, name string, patch map[string]any) map[string]any
	Remove(name string) map[string]any
	Restart(ctx context.Context, name string) map[string]any
}

// SetMCPAdmin enables the mcpServers global.
func (v *VM) SetMCPAdmin(a MCPAdmin) { v.mcpAdmin.Store(&a) }

func (v *VM) admin() MCPAdmin {
	if p := v.mcpAdmin.Load(); p != nil {
		return *p
	}
	return nil
}

// callCtx is the context of the running Exec, bounded by the js timeout.
func (v *VM) callCtx() (context.Context, context.CancelFunc) {
	ctx := v.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if v.active > 0 {
		return context.WithTimeout(ctx, v.active)
	}
	return context.WithCancel(ctx)
}

func (v *VM) bindMCPAdmin() {
	rt := v.rt
	obj := rt.NewObject()
	need := func() MCPAdmin {
		a := v.admin()
		if a == nil {
			panic(rt.NewGoError(errNoAdmin))
		}
		return a
	}
	obj2 := func(call goja.FunctionCall, i int, what string) map[string]any {
		if len(call.Arguments) <= i || goja.IsUndefined(call.Arguments[i]) || goja.IsNull(call.Arguments[i]) {
			return map[string]any{}
		}
		m, ok := call.Arguments[i].Export().(map[string]any)
		if !ok {
			panic(rt.NewTypeError("mcpServers.%s: expected an object", what))
		}
		return m
	}
	name := func(call goja.FunctionCall, what string) string {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("mcpServers.%s: the server name is required", what))
		}
		return call.Arguments[0].String()
	}
	_ = obj.Set("list", func() any { return need().List() })
	_ = obj.Set("add", func(call goja.FunctionCall) goja.Value {
		spec := obj2(call, 0, "add")
		ctx, cancel := v.callCtx()
		defer cancel()
		defer v.syncMCP()
		return rt.ToValue(need().Add(ctx, spec))
	})
	_ = obj.Set("update", func(call goja.FunctionCall) goja.Value {
		n, patch := name(call, "update"), obj2(call, 1, "update")
		ctx, cancel := v.callCtx()
		defer cancel()
		defer v.syncMCP()
		return rt.ToValue(need().Update(ctx, n, patch))
	})
	_ = obj.Set("remove", func(call goja.FunctionCall) goja.Value {
		defer v.syncMCP()
		return rt.ToValue(need().Remove(name(call, "remove")))
	})
	_ = obj.Set("restart", func(call goja.FunctionCall) goja.Value {
		n := name(call, "restart")
		ctx, cancel := v.callCtx()
		defer cancel()
		defer v.syncMCP()
		return rt.ToValue(need().Restart(ctx, n))
	})
	_ = rt.GlobalObject().DefineDataProperty("mcpServers", obj, goja.FLAG_TRUE, goja.FLAG_FALSE, goja.FLAG_FALSE)
}

type adminErr string

func (e adminErr) Error() string { return string(e) }

const errNoAdmin = adminErr("mcpServers: MCP management is not available")
