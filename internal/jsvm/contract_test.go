package jsvm

import (
	"context"
	"strings"
	"testing"
)

func ex(t *testing.T, v *VM, code string) Result {
	t.Helper()
	return v.Exec(context.Background(), code)
}

func TestReturnAndLastExpression(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	cases := map[string]string{
		"1 + 2":                                 "3",
		"return 5":                              "5",
		"const a = 2; return a * 3":             "6",
		"const x = await Promise.resolve(7); x": "7",
		"const x = await Promise.resolve(7); return x + 1": "8",
		"if (true) { 1 } else { 2 }":                       "1",
		"'a string'":                                       "a string",
		"null":                                             "null",
		"[1,2]":                                            "[\n  1,\n  2\n]",
		"({a: {b: 1}})":                                    "{\n  \"a\": {\n    \"b\": 1\n  }\n}",
		"true":                                             "true",
		"// await in a comment\n 'x' + \"await\"": "xawait",
	}
	for code, want := range cases {
		if r := ex(t, v, code); r.Err != "" || r.Value != want {
			t.Errorf("%q => %+v, want %q", code, r, want)
		}
	}
	if r := ex(t, v, "console.log('printed'); return 'value'"); r.Output != "printed\n" || r.Value != "value" {
		t.Fatalf("%+v", r)
	}
	if r := ex(t, v, "undefined"); r.Value != "" || r.Err != "" {
		t.Fatalf("%+v", r)
	}
}

func TestUnsupportedResultsFailExplicitly(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	for code, frag := range map[string]string{
		"(function f(){})":               "function",
		"new Map([[1,2]])":               "Map",
		"new Set([1])":                   "Set",
		"Symbol('s')":                    "Symbol",
		"const o = {}; o.o = o; o":       "JSON",
		"({toJSON(){return undefined}})": "JSON",
	} {
		r := ex(t, v, code)
		if !strings.Contains(r.Err, frag) || strings.Contains(r.Value, "[object") {
			t.Errorf("%q => %+v", code, r)
		}
	}
	// A patched JSON.stringify does not change how results are shown.
	if r := ex(t, v, "JSON.stringify = () => 'hacked'; ({a:1})"); !strings.Contains(r.Value, "\"a\": 1") {
		t.Fatalf("%+v", r)
	}
}

func TestPendingPromiseFailsAndDoesNotOutliveCall(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	r := ex(t, v, "await new Promise(() => {})")
	if !strings.Contains(r.Err, "pending") {
		t.Fatalf("%+v", r)
	}
	if r := ex(t, v, "await Promise.reject(new Error('nope'))"); !strings.Contains(r.Err, "nope") {
		t.Fatalf("%+v", r)
	}
}

func TestSyntaxErrorPointsAtRealProblem(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	r := ex(t, v, "const a = 1;\nconst b = await 2;\nlet = ;")
	if r.Err == "" || !strings.Contains(r.Err, "Line 3") {
		t.Fatalf("%+v", r)
	}
}

func TestRemovedGlobalsAreGone(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	for _, name := range []string{"state", "sessions", "__state"} {
		if r := ex(t, v, "typeof "+name); r.Value != "undefined" && r.Output != "undefined" {
			t.Fatalf("%s must not exist: %+v", name, r)
		}
	}
}
