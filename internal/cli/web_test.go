package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runWebCLI(t *testing.T, f *fakeAPI, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(append([]string{"meno", "web"}, args...), IO{Out: &out, Err: &errb}, f)
	return code, out.String(), errb.String()
}

func TestWebFetchPrintsFinalURLTruncationAndTrustNotice(t *testing.T) {
	f := &fakeAPI{}
	code, out, _ := runWebCLI(t, f, "fetch", "https://a.test/x")
	if code != 0 || !strings.Contains(out, "Title: T") || !strings.Contains(out, "[truncated: cut]") || !strings.Contains(out, "untrusted") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestWebSearchPrintsSources(t *testing.T) {
	code, out, _ := runWebCLI(t, &fakeAPI{}, "search", "go", "generics")
	if code != 0 || !strings.Contains(out, "an answer") || !strings.Contains(out, "https://x.test") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestWebUsageErrorsAndForbidden(t *testing.T) {
	for _, args := range [][]string{{}, {"search"}, {"fetch"}, {"fetch", "a", "b"}} {
		if code, _, _ := runWebCLI(t, &fakeAPI{}, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	if code, _, _ := runWebCLI(t, &fakeAPI{forbidden: true}, "fetch", "http://x"); code != 3 {
		t.Errorf("forbidden exit %d, want 3", code)
	}
}

func TestWebJSON(t *testing.T) {
	_, out, _ := runWebCLI(t, &fakeAPI{}, "fetch", "https://a.test", "--json")
	if !strings.Contains(out, `"final_url":"https://a.test"`) {
		t.Fatal(out)
	}
}

func TestWebUnknownSubcommandFails(t *testing.T) {
	if code, _, _ := runWebCLI(t, &fakeAPI{}, "bogus"); code == 0 {
		t.Fatal("unknown subcommand must fail")
	}
}
