package jsvm

import "github.com/dop251/goja"

// Policy says which of meno's tool globals a run keeps. A mode sets it before
// the first call of a request; the runtime is built with it (see restrict).
type Policy struct {
	// Only lists the tool globals the run keeps. Nil keeps all of them. The
	// JavaScript language itself (Math, JSON, ...) is never removed.
	Only []string
}

// ToolGlobals are the globals meno adds to the runtime. A policy removes the
// ones it does not keep.
var ToolGlobals = []string{
	"fs", "sh", "view", "websearch", "tasks", "agents", "session", "mcp", "mcpTools",
	"mcpResults", "mcpServers", "goal", "help", "askUser", "hash", "path", "base64", "btoa", "atob",
}

// SetPolicy sets the policy for the following calls. It takes effect when the
// runtime is built, which the next request does (see Reset).
func (v *VM) SetPolicy(p Policy) { v.policy.Store(&p) }

func (v *VM) currentPolicy() Policy {
	if p := v.policy.Load(); p != nil {
		return *p
	}
	return Policy{}
}

// restrict removes the tool globals the policy does not keep. bind calls it
// last, so no call of the run sees a global its mode does not allow.
func (v *VM) restrict(rt *goja.Runtime) {
	p := v.currentPolicy()
	if p.Only == nil {
		return
	}
	keep := make(map[string]bool, len(p.Only))
	for _, n := range p.Only {
		keep[n] = true
	}
	g := rt.GlobalObject()
	for _, n := range ToolGlobals {
		if !keep[n] {
			_ = g.Delete(n)
		}
	}
}

// Only is the list of tool globals the current policy keeps, nil when it keeps
// them all. The description of the js tool follows it (prompt.ToolDescriptionFor).
func (v *VM) Only() []string { return v.currentPolicy().Only }
