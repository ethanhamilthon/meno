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

// MCPResult is a tool result as the VM shows it. Text is only the text blocks,
// Structured only explicit server data (nil otherwise). Large parts are cut and
// Truncated is set; the complete result stays readable under ResultID.
type MCPResult struct {
	Text       string
	Structured json.RawMessage
	Content    []json.RawMessage
	Artifacts  []map[string]any
	IsError    bool
	ResultID   string
	Truncated  bool
	Notes      []string
}

// MCPHost connects the VM to MCP servers. Call fails for transport and
// protocol errors; a tool's own isError comes back in the result.
type MCPHost interface {
	Tools() []MCPTool
	Call(ctx context.Context, server, tool string, args map[string]any) (MCPResult, error)
	ResultInfo(id string) (any, error)
	ResultRead(id string, offset, limit int) (any, error)
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
			res, err := v.host().Call(ctx, t.Server, t.Name, args)
			if err != nil {
				panic(rt.NewGoError(fmt.Errorf("mcp.%s.%s: %w", t.Server, t.Name, err)))
			}
			return rt.ToValue(envelope(res))
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

// jsonValue decodes raw JSON into plain values; nil raw is null.
func jsonValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	return v
}

// envelope is the object mcp.<server>.<tool>() returns.
func envelope(r MCPResult) map[string]any {
	content := make([]any, len(r.Content))
	for i, c := range r.Content {
		content[i] = jsonValue(c)
	}
	arts := make([]any, len(r.Artifacts))
	for i, a := range r.Artifacts {
		arts[i] = a
	}
	out := map[string]any{
		"text": r.Text, "structuredContent": jsonValue(r.Structured), "content": content,
		"artifacts": arts, "isError": r.IsError, "resultId": r.ResultID, "truncated": r.Truncated,
	}
	if len(r.Notes) > 0 {
		out["notes"] = r.Notes
	}
	return out
}

// bindMCPResults installs mcpResults.info and mcpResults.read.
func (v *VM) bindMCPResults() {
	rt := v.rt
	obj := rt.NewObject()
	host := func() MCPHost {
		h := v.host()
		if h == nil {
			panic(rt.NewGoError(fmt.Errorf("mcp results are not available here")))
		}
		return h
	}
	check := func(res any, err error) goja.Value {
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(res)
	}
	_ = obj.Set("info", func(c goja.FunctionCall) goja.Value {
		return check(host().ResultInfo(strArg(rt, c.Arguments, 0, "id")))
	})
	_ = obj.Set("read", func(c goja.FunctionCall) goja.Value {
		id := strArg(rt, c.Arguments, 0, "id")
		o := optsArg(rt, c.Arguments, 1)
		checkKeys(rt, "mcpResults.read", o, "offset", "limit")
		return check(host().ResultRead(id, intOpt(rt, o, "offset"), intOpt(rt, o, "limit")))
	})
	_ = rt.Set("mcpResults", obj)
}
