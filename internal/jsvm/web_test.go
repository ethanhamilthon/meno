package jsvm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestEnvIsVisibleToShell(t *testing.T) {
	t.Setenv("MENO_TEST_ENV", "from-process")
	v := New(Options{Root: t.TempDir()})
	r := run(t, v, "env.MENO_TEST_ENV")
	if r.Value != "from-process" {
		t.Fatalf("env: %q", r.Value)
	}
	r = run(t, v, "env.MENO_JS_SET = 'from-js'; sh('echo $MENO_JS_SET $MENO_TEST_ENV').stdout.trim()")
	if r.Value != "from-js from-process" {
		t.Fatalf("shell env: %q", r.Value)
	}
	if os.Getenv("MENO_JS_SET") != "" {
		t.Fatal("js env leaked into the process")
	}
}

func TestURL(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	r := run(t, v, `const u = new URL('https://user:pw@Example.com:8080/a/b?x=1&y=two%20words#frag');
[u.protocol, u.host, u.hostname, u.port, u.pathname, u.search, u.hash, u.origin, u.username, u.searchParams.get('y')].join('|')`)
	want := "https:|example.com:8080|example.com|8080|/a/b|?x=1&y=two%20words|#frag|https://example.com:8080|user|two words"
	if r.Value != want {
		t.Fatalf("got %q\nwant %q", r.Value, want)
	}
	r = run(t, v, `const u = new URL('https://user:pw@Example.com:8080/a/b?x=1&y=two%20words#frag'); u.searchParams.append('z', 'a b&c'); u.searchParams.set('x', '9'); u.searchParams.delete('y'); u.href`)
	if r.Value != "https://user:pw@example.com:8080/a/b?x=9&z=a+b%26c#frag" {
		t.Fatalf("href: %q", r.Value)
	}
	r = run(t, v, `new URL('../c?q=1', 'http://h.test/a/b/d').href + ' ' + new URL('http://h.test').pathname + ' ' + URL.canParse('nope') + ' ' + [...new URLSearchParams({a: '1', b: '2'})].join(';') + ' ' + JSON.stringify(new URL('file:///tmp/x'))`)
	if r.Value != "http://h.test/a/c?q=1 / false a,1;b,2 \"file:///tmp/x\"" {
		t.Fatalf("misc: %q", r.Value)
	}
	if r := v.Exec(context.Background(), "new URL('not a url')"); !strings.Contains(r.Err, "Invalid URL") {
		t.Fatalf("invalid: %+v", r)
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/echo":
			b, _ := io.ReadAll(r.Body)
			w.Header().Set("X-Seen", r.Method+" "+r.Header.Get("X-Test")+" "+r.Header.Get("Content-Type"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"body":` + string(mustQuote(string(b))) + `}`))
		case "/missing":
			http.NotFound(w, r)
		case "/go":
			http.Redirect(w, r, "/echo", http.StatusFound)
		}
	}))
	defer srv.Close()
	v := New(Options{Root: t.TempDir()})
	pre := "const base = " + string(mustQuote(srv.URL)) + ";\n"
	r := run(t, v, pre+`const res = fetch(base + '/echo', {method: 'POST', headers: {'X-Test': 'yes'}, body: new URLSearchParams({a: '1'})});
[res.ok, res.status, res.headers.get('x-seen'), res.json().body].join('|')`)
	if r.Value != "true|200|POST yes application/x-www-form-urlencoded;charset=UTF-8|a=1" {
		t.Fatalf("post: %q", r.Value)
	}
	r = run(t, v, pre+"const m = fetch(base + '/missing'); [m.ok, m.status, m.statusText, m.text().trim()].join('|')")
	if r.Value != "false|404|Not Found|404 page not found" {
		t.Fatalf("404: %q", r.Value)
	}
	// await is optional: awaiting plain values works too.
	r = run(t, v, pre+"const rr = await fetch(base + '/go'); const j = await rr.json(); return [rr.redirected, new URL(rr.url).pathname, typeof j.body].join('|')")
	if r.Value != "true|/echo|string" {
		t.Fatalf("redirect: %q", r.Value)
	}
	if r := v.Exec(context.Background(), "fetch('http://127.0.0.1:1/')"); r.Err == "" {
		t.Fatal("connection error not reported")
	}
}

func mustQuote(s string) []byte {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		if c == '"' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	b.WriteByte('"')
	return []byte(b.String())
}
