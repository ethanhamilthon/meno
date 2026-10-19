package jsvm

import "github.com/dop251/goja"

// intOpt reads an optional non-negative integer option.
func intOpt(rt *goja.Runtime, o map[string]any, key string) int {
	v, ok := o[key]
	if !ok || v == nil {
		return 0
	}
	n, ok := v.(int64)
	if !ok || n < 0 {
		panic(rt.NewTypeError("%s must be a positive integer", key))
	}
	return int(n)
}

func checkKeys(rt *goja.Runtime, what string, o map[string]any, allowed ...string) {
	for k := range o {
		ok := false
		for _, a := range allowed {
			ok = ok || a == k
		}
		if !ok {
			panic(rt.NewTypeError("%s: unknown option %q (allowed: %v)", what, k, allowed))
		}
	}
}

func lineRangeOptions(rt *goja.Runtime, what string, o map[string]any) (int, int) {
	if alias, ok := o["offset"]; ok {
		if _, hasLong := o["startLine"]; hasLong {
			panic(rt.NewTypeError("%s: cannot use both \"offset\" and \"startLine\"", what))
		}
		if n, valid := alias.(int64); !valid || n < 0 || n > int64(int(^uint(0)>>1))-1 {
			panic(rt.NewTypeError("%s: offset must be a non-negative integer smaller than the maximum line number", what))
		}
	}
	if _, ok := o["limit"]; ok {
		if _, hasLong := o["endLine"]; hasLong {
			panic(rt.NewTypeError("%s: cannot use both \"limit\" and \"endLine\"", what))
		}
	}
	start := intOpt(rt, o, "startLine")
	if _, ok := o["offset"]; ok {
		start = intOpt(rt, o, "offset")
		if start == 0 {
			start = 1
		}
	}
	end := intOpt(rt, o, "endLine")
	if _, ok := o["limit"]; ok {
		limit := intOpt(rt, o, "limit")
		if limit < 1 {
			panic(rt.NewTypeError("%s: limit must be a positive integer", what))
		}
		if start == 0 {
			start = 1
		}
		if limit-1 > int(^uint(0)>>1)-start {
			panic(rt.NewTypeError("%s: offset + limit is too large", what))
		}
		end = start + limit - 1
	}
	return start, end
}

func (v *VM) bindFS() {
	rt := v.rt
	fsh := func() FS { return FS{Root: v.Root()} }
	throw := func(err error) {
		if err != nil {
			panic(rt.NewGoError(err))
		}
	}
	obj := rt.NewObject()
	_ = obj.Set("read", func(c goja.FunctionCall) goja.Value {
		p := strArg(rt, c.Arguments, 0, "path")
		o := optsArg(rt, c.Arguments, 1)
		checkKeys(rt, "fs.read", o, "startLine", "endLine", "offset", "limit")
		if len(o) == 0 {
			s, err := fsh().Read(p)
			throw(err)
			return rt.ToValue(s)
		}
		start, end := lineRangeOptions(rt, "fs.read", o)
		s, err := fsh().ReadRange(p, start, end)
		throw(err)
		return rt.ToValue(s)
	})
	_ = obj.Set("inspect", func(c goja.FunctionCall) goja.Value {
		p := strArg(rt, c.Arguments, 0, "path")
		o := optsArg(rt, c.Arguments, 1)
		checkKeys(rt, "fs.inspect", o, "startLine", "endLine", "offset", "limit")
		start, end := lineRangeOptions(rt, "fs.inspect", o)
		r, err := fsh().Inspect(p, start, end)
		throw(err)
		return rt.ToValue(map[string]any{"text": r.Text, "sha256": r.SHA256, "startLine": r.StartLine, "endLine": r.EndLine})
	})
	_ = obj.Set("search", func(c goja.FunctionCall) goja.Value {
		q := strArg(rt, c.Arguments, 0, "query")
		o := optsArg(rt, c.Arguments, 1)
		checkKeys(rt, "fs.search", o, "root", "glob", "regex", "caseSensitive", "includeIgnored", "limit", "cursor")
		so := SearchOptions{Limit: intOpt(rt, o, "limit")}
		so.Root, _ = o["root"].(string)
		so.Glob, _ = o["glob"].(string)
		so.Cursor, _ = o["cursor"].(string)
		so.Regex, _ = o["regex"].(bool)
		so.IncludeIgnored, _ = o["includeIgnored"].(bool)
		if cs, ok := o["caseSensitive"].(bool); ok {
			so.CaseInsensitive = !cs
		}
		r, err := fsh().Search(q, so)
		throw(err)
		ms := make([]any, len(r.Matches))
		for i, m := range r.Matches {
			ms[i] = map[string]any{"path": m.Path, "line": m.Line, "column": m.Column, "text": m.Text}
		}
		var next any
		if r.NextCursor != "" {
			next = r.NextCursor
		}
		return rt.ToValue(map[string]any{"matches": ms, "nextCursor": next, "truncated": r.Truncated})
	})
	_ = obj.Set("patch", func(c goja.FunctionCall) goja.Value {
		p := strArg(rt, c.Arguments, 0, "path")
		if len(c.Arguments) < 2 {
			panic(rt.NewTypeError("edits are required"))
		}
		list, ok := c.Arguments[1].Export().([]any)
		if !ok {
			panic(rt.NewTypeError("edits must be an array of {oldText, newText}"))
		}
		var edits []Replacement
		for i, e := range list {
			m, ok := e.(map[string]any)
			old, ok1 := m["oldText"].(string)
			nw, ok2 := m["newText"].(string)
			if !ok || !ok1 || !ok2 {
				panic(rt.NewTypeError("edit %d must be {oldText: string, newText: string}", i))
			}
			edits = append(edits, Replacement{old, nw})
		}
		o := optsArg(rt, c.Arguments, 2)
		checkKeys(rt, "fs.patch", o, "expectedHash")
		hash, _ := o["expectedHash"].(string)
		sum, n, err := fsh().Patch(p, edits, hash)
		throw(err)
		return rt.ToValue(map[string]any{"sha256": sum, "replacements": n})
	})
	_ = obj.Set("write", func(c goja.FunctionCall) goja.Value {
		p := strArg(rt, c.Arguments, 0, "path")
		content := strArg(rt, c.Arguments, 1, "content")
		overwrite := false
		if len(c.Arguments) > 2 && !goja.IsUndefined(c.Arguments[2]) && !goja.IsNull(c.Arguments[2]) {
			if b, ok := c.Arguments[2].Export().(bool); ok {
				overwrite = b
			} else {
				o := optsArg(rt, c.Arguments, 2)
				checkKeys(rt, "fs.write", o, "overwrite")
				overwrite, _ = o["overwrite"].(bool)
			}
		}
		throw(fsh().Write(p, content, overwrite))
		return goja.Undefined()
	})
	_ = obj.Set("glob", func(p string) ([]string, error) { return fsh().Glob(p) })
	_ = obj.Set("exists", func(p string) bool { return fsh().Exists(p) })
	_ = rt.Set("fs", obj)
}
