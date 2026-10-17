package llm

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestMetadataParsingAndMatching(t *testing.T) {
	m, err := decodeMetadata(strings.NewReader(`{"openai":{"models":{"test":{"limit":{"context":128000},"cost":{"input":2,"output":10,"cache_read":0.5}},"no-cost":{"limit":{"context":100}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	info, ok := MatchMetadata(m, Provider{Name: "custom", BaseURL: "https://api.openai.com/v1"}, "test")
	if !ok || info.Context != 128000 {
		t.Fatal("metadata not matched")
	}
	if _, ok := MatchMetadata(m, Provider{Name: "custom", BaseURL: "https://proxy.example/v1"}, "test"); ok {
		t.Fatal("guessed price")
	}
	if _, ok := m["openai/no-cost"]; ok {
		t.Fatal("missing price treated as free")
	}
	cost := info.Cost(Usage{In: 1000, Out: 100, Cached: 600})
	if math.Abs(cost-0.0021) > 1e-10 {
		t.Fatalf("cost %f", cost)
	}
}
func TestMetadataRequiresCompletePricing(t *testing.T) {
	for _, tc := range []struct {
		name string
		cost string
		want bool
	}{
		{"absent", "", false},
		{"null", `,"cost":null`, false},
		{"empty", `,"cost":{}`, false},
		{"input-only", `,"cost":{"input":2}`, false},
		{"output-only", `,"cost":{"output":10}`, false},
		{"null-input", `,"cost":{"input":null,"output":10}`, false},
		{"null-output", `,"cost":{"input":2,"output":null}`, false},
		{"cache-only", `,"cost":{"cache_read":0,"cache_write":0}`, false},
		{"free", `,"cost":{"input":0,"output":0}`, true},
		{"paid", `,"cost":{"input":2,"output":10}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog, err := decodeMetadata(strings.NewReader(`{"openai":{"models":{"test":{"limit":{"context":100}` + tc.cost + `}}}}`))
			if err != nil {
				t.Fatal(err)
			}
			info, ok := catalog["openai/test"]
			if ok != tc.want {
				t.Fatalf("price present = %v, want %v", ok, tc.want)
			}
			if ok && (!info.CanPrice(Usage{In: 100, Out: 10}) || info.CanPrice(Usage{In: 100, CacheWrite: 10})) {
				t.Fatal("incorrect cache write price availability")
			}
			if tc.name == "free" && info.Cost(Usage{In: 100, Out: 10}) != 0 {
				t.Fatal("explicit free price lost")
			}
		})
	}
}

func TestModelInfoCanPrice(t *testing.T) {
	zero := 0.0
	for _, tc := range []struct {
		name string
		info ModelInfo
		u    Usage
		want bool
	}{
		{"manual-zero", ModelInfo{}, Usage{In: 100, Out: 10}, true},
		{"legacy-cache-read", ModelInfo{Input: 2, Output: 10}, Usage{In: 100, Cached: 50}, true},
		{"missing-write", ModelInfo{Input: 2, Output: 10}, Usage{In: 100, CacheWrite: 10}, false},
		{"write-only", ModelInfo{}, Usage{CacheWrite: 10}, false},
		{"clamped-write", ModelInfo{}, Usage{In: 100, Cached: 100, CacheWrite: 10}, false},
		{"negative-write", ModelInfo{}, Usage{CacheWrite: -1}, true},
		{"free-write", ModelInfo{CacheWrite: &zero}, Usage{In: 100, CacheWrite: 10}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.info.CanPrice(tc.u); got != tc.want {
				t.Fatalf("CanPrice(%+v) = %v, want %v", tc.u, got, tc.want)
			}
		})
	}
}

func TestResponsesCachedUsage(t *testing.T) {
	var usage Usage
	err := handleResponsesEvent(SSEEvent{Data: `{"type":"response.completed","response":{"usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":80}}}}`}, func(e Event) { usage = e.Usage })
	if err != nil || usage.Cached != 80 || usage.In != 100 {
		t.Fatalf("usage %+v %v", usage, err)
	}
}

func TestCacheWritePricing(t *testing.T) {
	catalog, err := decodeMetadata(strings.NewReader(`{"openai":{"models":{"test":{"cost":{"input":4,"output":20,"cache_read":0.4,"cache_write":5}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	info := catalog["openai/test"]
	if info.CacheWrite == nil || *info.CacheWrite != 5 {
		t.Fatal(info)
	}
	for _, tc := range []struct {
		usage Usage
		cost  float64
	}{
		{Usage{In: 1000, Out: 100, Cached: 600, CacheWrite: 300}, 0.00414},
		{Usage{In: 100, Cached: 200, CacheWrite: 200}, 0.00004},
		{Usage{In: 100, CacheWrite: 200}, 0.0005},
		{Usage{In: -1, Out: -1, Cached: -1, CacheWrite: -1}, 0},
	} {
		if got := info.Cost(tc.usage); math.Abs(got-tc.cost) > 1e-10 {
			t.Fatalf("%+v: %g != %g", tc.usage, got, tc.cost)
		}
	}
	info.CacheWrite = nil
	if got := info.Cost(Usage{In: 100, CacheWrite: 100}); math.Abs(got-0.0004) > 1e-10 {
		t.Fatal(got)
	}
	zero := 0.0
	info.CacheWrite = &zero
	if got := info.Cost(Usage{In: 100, CacheWrite: 100}); got != 0 {
		t.Fatal(got)
	}
}

func TestPublicCatalogs(t *testing.T) {
	router, err := decodeOpenRouter(strings.NewReader(`{"data":[{"id":"openai/test","context_length":128000,"pricing":{"prompt":"0.000002","completion":"0.00001","input_cache_read":"0.0000005"}},{"id":"free","pricing":{"prompt":"0","completion":"0"}},{"id":"unknown","context_length":4096,"pricing":{}},{"id":"invalid","pricing":{"prompt":"-1","completion":"NaN"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	info, ok := MatchMetadata(router, Provider{Name: "router", BaseURL: "https://openrouter.ai/api/v1"}, "openai/test")
	if !ok || info.Context != 128000 || info.Input != 2 || info.Output != 10 || info.CacheRead == nil || *info.CacheRead != 0.5 {
		t.Fatalf("router info: %+v", info)
	}
	if !router["openrouter/free"].CanPrice(Usage{In: 100}) || router["openrouter/unknown"].CanPrice(Usage{}) || router["openrouter/invalid"].CanPrice(Usage{}) {
		t.Fatal("missing/invalid prices treated as free")
	}
	lite, err := decodeLiteLLM(strings.NewReader(`{"test":{"litellm_provider":"openai","mode":"chat","max_input_tokens":64000,"input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"cache_read_input_token_cost":0.0000005,"cache_creation_input_token_cost":0.000003},"openrouter/openai/test":{"litellm_provider":"openrouter","mode":"chat","max_tokens":128000},"embed":{"litellm_provider":"openai","mode":"embedding"},"free":{"litellm_provider":"openai","mode":"chat","input_cost_per_token":0,"output_cost_per_token":0}}`))
	if err != nil {
		t.Fatal(err)
	}
	info = lite["openai/test"]
	if info.Context != 64000 || info.Input != 2 || info.Output != 10 || info.CacheWrite == nil || *info.CacheWrite != 3 {
		t.Fatalf("lite info: %+v", info)
	}
	if _, ok := lite["openai/embed"]; ok {
		t.Fatal("embedding offered as chat model")
	}
	if info := lite["openrouter/openai/test"]; info.Context != 128000 || info.CanPrice(Usage{}) {
		t.Fatal(info)
	}
	if !lite["openai/free"].CanPrice(Usage{}) {
		t.Fatal("free price lost")
	}
	models := CatalogModels(router, Provider{Name: "router", BaseURL: "https://openrouter.ai/api/v1"})
	if len(models) != 4 || models[2].ID != "openai/test" || models[2].Provider != "router" {
		t.Fatal(models)
	}
	if models := CatalogModels(lite, Provider{Name: "openai", BaseURL: "https://proxy.example/v1"}); len(models) != 0 {
		t.Fatal("guessed proxy catalog", models)
	}
}

func TestFetchMetadataPartialFailure(t *testing.T) {
	old := modelsHTTP
	defer func() { modelsHTTP = old }()
	modelsHTTP = &http.Client{Transport: metadataTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "openrouter.ai" {
			return nil, fmt.Errorf("offline")
		}
		if req.Header.Get("Authorization") != "" {
			t.Error("public request carried credentials")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"test":{"litellm_provider":"openai","mode":"chat","max_input_tokens":100,"input_cost_per_token":0,"output_cost_per_token":0}}`)), Header: make(http.Header)}, nil
	})}
	catalog, err := FetchMetadata(context.Background())
	if err != nil || catalog["openai/test"].Context != 100 {
		t.Fatalf("partial catalog: %v %v", catalog, err)
	}
}

type metadataTransport func(*http.Request) (*http.Response, error)

func (f metadataTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDecodeSurvivesNonNumericAndArrayFields(t *testing.T) {
	or, err := decodeOpenRouter(strings.NewReader(`{"data":[
	 {"id":"a/b","context_length":1000,"pricing":{"prompt":"0.000002","completion":"0.00001","overrides":[{"x":1}],"input_cache_read":"0.0000005"}},
	 {"id":"a/free","context_length":10,"pricing":{"prompt":"0","completion":"0"}},
	 {"id":"a/nopr","context_length":10,"pricing":{"prompt":"-1","completion":"0"}},
	 {"id":7}]}`))
	if err != nil {
		t.Fatal(err)
	}
	b := or["openrouter/a/b"]
	if b.Context != 1000 || math.Abs(b.Input-2) > 1e-9 || math.Abs(b.Output-10) > 1e-9 || b.CacheRead == nil || b.MissingPricing {
		t.Fatalf("openrouter %+v", b)
	}
	if f := or["openrouter/a/free"]; f.MissingPricing || f.Input != 0 {
		t.Fatalf("free %+v", f)
	}
	if !or["openrouter/a/nopr"].MissingPricing {
		t.Fatal("negative price accepted")
	}
	ll, err := decodeLiteLLM(strings.NewReader(`{"sample_spec":{"max_input_tokens":"doc string","mode":"chat","litellm_provider":"x"},
	 "gpt-x":{"litellm_provider":"openai","mode":"chat","max_input_tokens":128000,"input_cost_per_token":0.0000025,"output_cost_per_token":0.00001},
	 "emb":{"litellm_provider":"openai","mode":"embedding"}}`))
	if err != nil {
		t.Fatal(err)
	}
	g := ll["openai/gpt-x"]
	if g.Context != 128000 || math.Abs(g.Input-2.5) > 1e-9 || len(ll) != 1 {
		t.Fatalf("litellm %+v len=%d", g, len(ll))
	}
}

func TestLookupMetadataCustomEndpointFallback(t *testing.T) {
	catalog := map[string]ModelInfo{
		"anthropic/claude-sonnet-4-5":            {Context: 200000, Input: 3, Output: 15},
		"openrouter/anthropic/claude-sonnet-4.5": {Context: 1000000, Input: 3, Output: 15},
		"bedrock/claude-sonnet-4-5":              {Context: 1, Input: 99, Output: 99},
	}
	proxy := Provider{Name: "cliproxy", BaseURL: "http://127.0.0.1:8317/v1"}
	if _, ok := MatchMetadata(catalog, proxy, "claude-sonnet-4-5"); ok {
		t.Fatal("strict match must not guess")
	}
	info, ok := LookupMetadata(catalog, proxy, "claude-sonnet-4.5")
	if !ok || info.Context != 200000 {
		t.Fatalf("fallback %+v %v", info, ok)
	}
	if _, ok := LookupMetadata(catalog, proxy, "unknown-model"); ok {
		t.Fatal("matched unknown model")
	}
	// Known hosts never fall back to other providers.
	if _, ok := LookupMetadata(catalog, Provider{Name: "x", BaseURL: "https://api.openai.com/v1"}, "claude-sonnet-4-5"); ok {
		t.Fatal("known host inherited another provider")
	}
}
