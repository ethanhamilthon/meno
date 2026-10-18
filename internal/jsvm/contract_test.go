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

type memState struct {
	m    map[string]string
	fail bool
}

func (s *memState) Set(k, j string) error {
	if s.fail {
		return context.DeadlineExceeded
	}
	s.m[k] = j
	return nil
}
func (s *memState) Get(k string) (string, bool, error) { j, ok := s.m[k]; return j, ok, nil }
func (s *memState) Delete(k string) (bool, error)      { _, ok := s.m[k]; delete(s.m, k); return ok, nil }
func (s *memState) Entries() ([]StateEntry, error) {
	var out []StateEntry
	for k, j := range s.m {
		out = append(out, StateEntry{k, j})
	}
	for i := range out { // byte-wise order
		for j := i + 1; j < len(out); j++ {
			if out[j].Key < out[i].Key {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func TestStateRoundTripAndCopies(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	st := &memState{m: map[string]string{}}
	v.SetState(st)
	r := ex(t, v, `state.set("b/c.d", {n: [1, "x", null, true], z: 1.5}); state.set("a", null);
state.set("é", "ü"); [state.has("a"), state.get("a"), state.has("nope"), state.get("nope")]`)
	if r.Err != "" || !strings.Contains(r.Value, "true") {
		t.Fatalf("%+v", r)
	}
	// Survives into the next call (a fresh runtime), as independent copies.
	r = ex(t, v, `const o = state.get("b/c.d"); o.n.push(99); const again = state.get("b/c.d");
[again.n.length, state.listKeys().join("|"), state.delete("a"), state.delete("a"), state.listValues().length, state.listEntries()[0][0]].join(";")`)
	if r.Err != "" || r.Value != "4;a|b/c.d|é;true;false;2;b/c.d" {
		t.Fatalf("%+v", r)
	}
}

func TestStateRejectsNonJSONWithPath(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	st := &memState{m: map[string]string{"keep": "1"}}
	v.SetState(st)
	for code, frag := range map[string]string{
		"state.set('k', undefined)":                   "value is undefined",
		"state.set('k', {a: [1, , 3]})":               "value.a[1] is a hole",
		"state.set('k', {f(){}})":                     "value.f is a function",
		"state.set('k', {a: NaN})":                    "value.a is not a finite number",
		"state.set('k', Infinity)":                    "not a finite number",
		"state.set('k', 10n)":                         "BigInt",
		"state.set('k', new Date())":                  "not a plain object",
		"state.set('k', new Map())":                   "not a plain object",
		"const o = {}; o.self = o; state.set('k', o)": "value.self is a cycle",
		"state.set('k', {get x(){ return 1 }})":       "value.x is an accessor",
		"state.set('k', {[Symbol('s')]: 1})":          "symbol key",
		"state.set('k', {toJSON(){ return 1 }})":      "value.toJSON is a function",
		"state.set('', 1)":                            "non-empty string",
		"state.set(1, 1)":                             "non-empty string",
	} {
		r := ex(t, v, code)
		if !strings.Contains(r.Err, frag) {
			t.Errorf("%s => %+v, want %q", code, r, frag)
		}
	}
	if _, ok := st.m["k"]; ok || st.m["keep"] != "1" {
		t.Fatalf("a rejected write changed the store: %v", st.m)
	}
	// A failed commit throws and leaves the old value; earlier writes persist
	// past a later exception.
	ex(t, v, "state.set('ok', 1); throw new Error('after')")
	if st.m["ok"] != "1" {
		t.Fatal("a write before an exception must persist")
	}
	st.fail = true
	if r := ex(t, v, "state.set('ok', 2)"); r.Err == "" || st.m["ok"] != "1" {
		t.Fatalf("%+v %v", r, st.m)
	}
}

func TestStateUnavailableWithoutStore(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	if r := ex(t, v, "state.get('x')"); !strings.Contains(r.Err, "not available") {
		t.Fatalf("%+v", r)
	}
}
