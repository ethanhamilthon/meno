package jsvm

import (
	"fmt"

	"github.com/dop251/goja"
)

// captureStringify keeps the runtime's own JSON.stringify so that code which
// replaces it cannot change how results are shown.
func captureStringify(rt *goja.Runtime) goja.Callable {
	fn, _ := goja.AssertFunction(rt.Get("JSON").ToObject(rt).Get("stringify"))
	return fn
}

// render is the text of a returned value: strings as they are, other
// primitives by their usual text, objects and arrays as indented JSON. Values
// with no faithful text (functions, symbols, Map, Set, cycles) are an error
// instead of "[object Object]".
func (v *VM) render(val goja.Value) (out string, err error) {
	if ex := v.rt.Try(func() { out, err = v.renderValue(val) }); ex != nil {
		return "", fmt.Errorf("result cannot be shown: %s", ex.Error())
	}
	return out, err
}

func (v *VM) renderValue(val goja.Value) (string, error) {
	rt := v.rt
	if goja.IsNull(val) {
		return "null", nil
	}
	obj, isObj := val.(*goja.Object)
	if !isObj {
		if _, sym := val.(*goja.Symbol); sym {
			return "", fmt.Errorf("result is a Symbol and cannot be shown")
		}
		return val.String(), nil
	}
	cls := obj.ClassName()
	if c, ok := obj.Get("constructor").(*goja.Object); ok && cls == "Object" {
		cls = c.Get("name").String()
	}
	switch cls {
	case "Function":
		return "", fmt.Errorf("result is a function and cannot be shown; call it or return its data")
	case "Map", "Set", "WeakMap", "WeakSet", "Promise", "Symbol":
		return "", fmt.Errorf("result is a %s and cannot be shown as JSON; convert it to an array or object first", cls)
	case "Error":
		return val.String(), nil
	}
	var out goja.Value
	var callErr error
	if ex := rt.Try(func() {
		out, callErr = v.stringify(goja.Undefined(), val, goja.Null(), rt.ToValue(2))
	}); ex != nil {
		return "", fmt.Errorf("result cannot be shown as JSON: %s", ex.Error())
	}
	if callErr != nil {
		return "", fmt.Errorf("result cannot be shown as JSON: %s", callErr)
	}
	if out == nil || goja.IsUndefined(out) {
		return "", fmt.Errorf("result cannot be shown as JSON")
	}
	return out.String(), nil
}
