package jsvm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dop251/goja"
)

// MCPTool describes one tool of a running MCP server.
type MCPTool struct {
	Server, Name, Description string
	Schema                    json.RawMessage
}

// MCPHost connects the VM to MCP servers.
type MCPHost interface {
	Tools() []MCPTool
	Call(ctx context.Context, server, tool string, args map[string]any) (string, error)
}

// SetMCP makes the tools of host callable as mcp.<server>.<tool>({...}).
func (v *VM) SetMCP(h MCPHost) {
	v.mcpHost.Store(&h)
}

func (v *VM) host() MCPHost {
	if p := v.mcpHost.Load(); p != nil {
		return *p
	}
	return nil
}

// ident makes name usable as a property in dot notation.
func ident(name string) string {
	out := []rune(name)
	for i, r := range out {
		ok := r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9'
		if !ok {
			out[i] = '_'
		}
	}
	if len(out) == 0 {
		return "_"
	}
	return string(out)
}

// syncMCP rebuilds the mcp global and mcpTools() from the servers running now.
// Servers come and go while the VM lives, so every Exec starts with this.
func (v *VM) syncMCP() {
	rt := v.rt
	root := rt.NewObject()
	var tools []MCPTool
	if h := v.host(); h != nil {
		tools = h.Tools()
	}
	servers := map[string]*goja.Object{}
	info := make([]any, 0, len(tools))
	for _, t := range tools {
		t := t
		srv := servers[t.Server]
		if srv == nil {
			srv = rt.NewObject()
			servers[t.Server] = srv
			_ = root.Set(t.Server, srv)
			if id := ident(t.Server); id != t.Server {
				_ = root.Set(id, srv)
			}
		}
		fn := func(call goja.FunctionCall) goja.Value {
			var args map[string]any
			if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) && !goja.IsNull(call.Arguments[0]) {
				m, ok := call.Arguments[0].Export().(map[string]any)
				if !ok {
					panic(rt.NewTypeError("mcp.%s.%s: the argument must be an object", t.Server, t.Name))
				}
				args = m
			}
			ctx := v.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			if v.active > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, v.active)
				defer cancel()
			}
			text, err := v.host().Call(ctx, t.Server, t.Name, args)
			if err != nil {
				panic(rt.NewGoError(fmt.Errorf("mcp.%s.%s: %w", t.Server, t.Name, err)))
			}
			return rt.ToValue(text)
		}
		_ = srv.Set(t.Name, fn)
		if id := ident(t.Name); id != t.Name {
			_ = srv.Set(id, fn)
		}
		var schema any
		_ = json.Unmarshal(t.Schema, &schema)
		info = append(info, map[string]any{"server": t.Server, "name": t.Name, "description": t.Description, "schema": schema})
	}
	_ = rt.GlobalObject().Set("mcp", root)
	_ = rt.GlobalObject().Set("mcpTools", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 || goja.IsUndefined(call.Arguments[0]) {
			return rt.ToValue(info)
		}
		want := call.Arguments[0].String()
		out := []any{}
		for _, it := range info {
			if it.(map[string]any)["server"] == want {
				out = append(out, it)
			}
		}
		return rt.ToValue(out)
	})
}
