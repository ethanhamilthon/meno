package websearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func saveURL(t *testing.T, p *string) {
	t.Helper()
	saved := *p
	t.Cleanup(func() { *p = saved })
}

func TestBraveSearch(t *testing.T) {
	saveURL(t, &braveURL)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Subscription-Token") != "key" || r.URL.Query().Get("q") != "go lang" || r.URL.Query().Get("count") != "2" {
			t.Errorf("bad request: %s %v", r.Header.Get("X-Subscription-Token"), r.URL.Query())
		}
		w.Write([]byte(`{"web":{"results":[{"title":"Go &amp; stuff","url":"https://go.dev","description":"The <b>language</b>"}]}}`))
	}))
	defer srv.Close()
	braveURL = srv.URL
	ans, err := Search(context.Background(), Brave, "key", "go lang", 2)
	if err != nil || len(ans.Results) != 1 {
		t.Fatalf("%+v %v", ans, err)
	}
	r := ans.Results[0]
	if r.Title != "Go & stuff" || r.Snippet != "The language" {
		t.Fatalf("result %+v", r)
	}
}

func TestBraveNeedsAKey(t *testing.T) {
	if _, err := Search(context.Background(), Brave, "", "q", 3); err == nil || !strings.Contains(err.Error(), "needs a key") {
		t.Fatalf("error %v", err)
	}
}

func TestTavilySearch(t *testing.T) {
	saveURL(t, &tavilyURL)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("bad request %s %s", r.Method, r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"answer":"Use Go.","results":[{"title":"Go","url":"https://go.dev","content":"Fast"}]}`))
	}))
	defer srv.Close()
	tavilyURL = srv.URL
	ans, err := Search(context.Background(), Tavily, "key", "go", 1)
	if err != nil || ans.Answer != "Use Go." || len(ans.Results) != 1 || ans.Results[0].Snippet != "Fast" {
		t.Fatalf("%+v %v", ans, err)
	}
}

func TestDuckDuckGoSearch(t *testing.T) {
	saveURL(t, &duckURL)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev%2F&amp;rut=x">Go &amp; more</a>` +
			`<a class="result__snippet" href="y">The <b>Go</b> language</a>` +
			`<a class="result__a" href="javascript:void(0)">skip me</a>`))
	}))
	defer srv.Close()
	duckURL = srv.URL
	ans, err := Search(context.Background(), DuckDuckGo, "", "go", 5)
	if err != nil || len(ans.Results) != 1 {
		t.Fatalf("%+v %v", ans, err)
	}
	if ans.Results[0].URL != "https://go.dev/" || ans.Results[0].Title != "Go & more" || ans.Results[0].Snippet != "The Go language" {
		t.Fatalf("result %+v", ans.Results[0])
	}
}

func TestSearchErrors(t *testing.T) {
	if _, err := Search(context.Background(), DuckDuckGo, "", "  ", 1); err == nil {
		t.Fatal("empty query accepted")
	}
	if _, err := Search(context.Background(), "bing", "", "q", 1); err == nil {
		t.Fatal("unknown backend accepted")
	}
	saveURL(t, &duckURL)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("slow down"))
	}))
	defer srv.Close()
	duckURL = srv.URL
	if _, err := Search(context.Background(), DuckDuckGo, "", "q", 1); err == nil || !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("error %v", err)
	}
}
