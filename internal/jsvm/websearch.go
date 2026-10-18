package jsvm

import (
	"context"

	"github.com/dop251/goja"
)

// WebSearcher runs a web search for the code of the running call. The host
// decides the order of backends; see internal/app/websearch.go.
type WebSearcher interface {
	Search(ctx context.Context, query string, opts map[string]any) (any, error)
}

// SetWebSearch enables websearch(query, opts).
func (v *VM) SetWebSearch(h WebSearcher) { v.webHost.Store(&h) }

func (v *VM) bindWebSearch() {
	p := v.webHost.Load()
	if p == nil {
		return
	}
	h := *p
	rt := v.rt
	check := func(res any, err error) goja.Value {
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(res)
	}
	_ = rt.Set("websearch", func(c goja.FunctionCall) goja.Value {
		query := strArg(rt, c.Arguments, 0, "query")
		o := optsArg(rt, c.Arguments, 1)
		checkKeys(rt, "websearch", o, "count")
		if n, ok := o["count"]; ok {
			switch num := n.(type) {
			case int64:
				if num < 1 || num > 10 {
					panic(rt.NewTypeError("count must be between 1 and 10"))
				}
			case float64:
				if num < 1 || num > 10 || num != float64(int64(num)) {
					panic(rt.NewTypeError("count must be a whole number between 1 and 10"))
				}
			default:
				panic(rt.NewTypeError("count must be a number"))
			}
		}
		return check(h.Search(v.runCtx(), query, o))
	})
}
