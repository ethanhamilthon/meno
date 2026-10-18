package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"meno/internal/llm"
)

// searchIn runs a host search with the UI loop of the test handling onUI.
func searchIn(t *testing.T, a *App, h webSearchHost, query string) (any, error) {
	t.Helper()
	runUI(t, a)
	return h.Search(context.Background(), query, map[string]any{"count": int64(2)})
}

func TestWebsearchOff(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	a.st.Settings.WebSearch = "off"
	if _, err := searchIn(t, a, webSearchHost{a: a, sid: a.st.Session.ID}, "go"); err == nil {
		t.Fatal("search ran while off")
	}
}

func TestWebsearchUsesProviderNativeSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"output":[` +
			`{"type":"web_search_call","action":{"type":"search","sources":[{"type":"url","url":"https://example.com/a"}]}},` +
			`{"type":"message","content":[{"type":"output_text","text":"Answer text",` +
			`"annotations":[{"type":"url_citation","url":"https://go.dev","title":"Go","start_index":0,"end_index":6}]}]}]}`))
	}))
	defer srv.Close()
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	a.st.Providers = []llm.Provider{{Name: "responses", BaseURL: srv.URL, APIKey: "k", Protocol: llm.ProtoResponses}}
	a.st.Settings.WebSearch = "auto"
	res, err := searchIn(t, a, webSearchHost{a: a, provider: "responses", model: "m"}, "go")
	if err != nil {
		t.Fatal(err)
	}
	v, ok := res.(map[string]any)
	if !ok || v["backend"] != "native" || v["answer"] != "Answer text" {
		t.Fatalf("result %+v", res)
	}
	rs, ok := v["results"].([]any)
	if !ok || len(rs) != 2 {
		t.Fatalf("results %+v", v["results"])
	}
}
