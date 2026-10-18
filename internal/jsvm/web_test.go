package jsvm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func run(t *testing.T, v *VM, code string) Result {
	t.Helper()
	r := v.Exec(context.Background(), code)
	if r.Err != "" {
		t.Fatalf("%s\n=> error: %s", code, r.Err)
	}
	return r
}

func TestConsoleReplacesPrint(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	r := run(t, v, "console.log('a', 1, {x: [1]}); console.error('e'); console.warn(undefined, null, true)")
	for _, want := range []string{"a 1 {\n  \"x\": [\n    1\n  ]\n}\n", "e\n", "undefined null true\n"} {
		if !strings.Contains(r.Output, want) {
			t.Fatalf("output %q misses %q", r.Output, want)
		}
	}
	if r := v.Exec(context.Background(), "typeof print + typeof __write + typeof __fetch + typeof __parseURL"); r.Value != "undefinedundefinedundefinedundefined" {
		t.Fatalf("leaked globals: %q %q", r.Value, r.Err)
	}
}

func TestWebGlobalsAreGone(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	r := run(t, v, "[typeof fetch, typeof URL, typeof URLSearchParams, typeof env, typeof Headers, typeof Response].join(' ')")
	if r.Value != "undefined undefined undefined undefined undefined undefined" {
		t.Fatalf("web globals still exist: %q", r.Value)
	}
}

func TestShellSeesTheProcessEnv(t *testing.T) {
	t.Setenv("MENO_TEST_ENV", "from-process")
	v := New(Options{Root: t.TempDir()})
	r := run(t, v, "sh('echo $MENO_TEST_ENV').stdout.trim()")
	if r.Value != "from-process" {
		t.Fatalf("shell env: %q", r.Value)
	}
}

type fakeWebSearch struct {
	query string
	opts  map[string]any
	res   any
	err   error
}

func (f *fakeWebSearch) Search(_ context.Context, query string, opts map[string]any) (any, error) {
	f.query, f.opts = query, opts
	return f.res, f.err
}

func TestWebsearchCallsTheHost(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	h := &fakeWebSearch{res: map[string]any{
		"query": "golang", "backend": "duckduckgo",
		"results": []any{map[string]any{"title": "Go", "url": "https://go.dev", "snippet": "The language"}},
	}}
	v.SetWebSearch(h)
	r := run(t, v, "JSON.stringify(websearch('  golang  ', {count: 3}))")
	if h.query != "  golang  " || h.opts["count"] != int64(3) {
		t.Fatalf("host got %q %+v", h.query, h.opts)
	}
	for _, want := range []string{"\"backend\":\"duckduckgo\"", "\"url\":\"https://go.dev\""} {
		if !strings.Contains(r.Value, want) {
			t.Fatalf("value %q misses %s", r.Value, want)
		}
	}
}

func TestWebsearchValidatesArguments(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	v.SetWebSearch(&fakeWebSearch{res: map[string]any{"results": []any{}}})
	for _, code := range []string{
		"websearch()",
		"websearch(1)",
		"websearch('q', {count: 0})",
		"websearch('q', {count: 11})",
		"websearch('q', {count: 'three'})",
		"websearch('q', {depth: 2})",
	} {
		if r := v.Exec(context.Background(), code); r.Err == "" {
			t.Errorf("%s: expected an error", code)
		}
	}
}

func TestWebsearchReportsHostErrors(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	v.SetWebSearch(&fakeWebSearch{err: errors.New("no backend")})
	if r := v.Exec(context.Background(), "websearch('q')"); !strings.Contains(r.Err, "no backend") {
		t.Fatalf("error %q", r.Err)
	}
}

func TestWebsearchNeedsAHost(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	if r := v.Exec(context.Background(), "typeof websearch"); r.Value != "undefined" {
		t.Fatalf("websearch without a host: %q", r.Value)
	}
}
