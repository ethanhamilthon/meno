package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestCacheOptions(t *testing.T) {
	for _, tc := range []struct {
		model, endpoint string
		options         *CacheOptions
		retention       string
		modern, key     bool
	}{
		{model: "gpt-5.4", endpoint: "https://api.openai.com/v1", retention: "24h", key: true},
		{model: "gpt-5.1-codex-max", endpoint: "https://api.openai.com/v1", retention: "24h", key: true},
		{model: "gpt-4.1-2025-04-14", endpoint: "https://api.openai.com/v1", retention: "24h", key: true},
		{model: "gpt-4o", endpoint: "https://api.openai.com/v1", key: true},
		{model: "gpt-5.6", endpoint: "https://api.openai.com/v1", modern: true},
		{model: "gpt-5.7-2026-10-01", endpoint: "https://api.openai.com/v1"},
		{model: "gpt-6", endpoint: "https://api.openai.com/v1", modern: true},
		{model: "gpt-6.1-sol", endpoint: "https://api.openai.com/v1", modern: true},
		{model: "gpt-6-astra", endpoint: "https://api.openai.com/v1", modern: true},
		{model: "gpt-6.2", endpoint: "https://api.openai.com/v1"},
		{model: "gpt-7", endpoint: "https://api.openai.com/v1"},
		{model: "gpt-5.4-unknown", endpoint: "https://api.openai.com/v1"},
		{model: "gpt-6-custom", endpoint: "https://api.openai.com/v1"},
		{model: "unknown", endpoint: "https://api.openai.com/v1"},
		{model: "unknown", endpoint: "https://api.openai.com/v1", options: &CacheOptions{}},
		{model: "unknown", endpoint: "https://proxy.example/v1", options: &CacheOptions{Key: "caller"}, key: true},
		{model: "unknown", endpoint: "https://api.openai.com/v1", options: &CacheOptions{Retention: "24h"}, retention: "24h"},
		{model: "unknown", endpoint: "https://proxy.example/v1", options: &CacheOptions{TTL: "30m"}, modern: true},
		{model: "unknown", endpoint: "https://api.openai.com/v1", options: &CacheOptions{Retention: "invalid"}},
		{model: "unknown", endpoint: "https://api.openai.com/v1", options: &CacheOptions{TTL: "24h"}},
		{model: "unknown", endpoint: "https://api.openai.com/v1", options: &CacheOptions{Mode: "explicit"}},
		{model: "unknown", endpoint: "https://api.openai.com/v1", options: &CacheOptions{Key: strings.Repeat("x", 65)}},
		{model: "gpt-5.5-pro", endpoint: "https://api.openai.com/v1", options: &CacheOptions{Retention: "in_memory"}, key: true},
		{model: "gpt-6.1-sol", endpoint: "https://api.openai.com/v1", options: &CacheOptions{TTL: "24h"}},
		{model: "gpt-6.1-sol", endpoint: "https://api.openai.com/v1", options: &CacheOptions{Disabled: true}},
		{model: "gpt-5.4", endpoint: "https://proxy.example/v1"},
		{model: "gpt-5.4", endpoint: "https://api.openai.com.evil.example/v1"},
		{model: "gpt-5.4", endpoint: "https://proxy.example/v1", options: &CacheOptions{}, retention: "24h", key: true},
		{model: "gpt-5.4", endpoint: "https://api.openai.com/v1", options: &CacheOptions{Disabled: true}},
	} {
		t.Run(tc.model+tc.endpoint, func(t *testing.T) {
			req := Request{Provider: Provider{Name: "openai", BaseURL: tc.endpoint}, Model: tc.model, System: "stable prefix", Cache: tc.options}
			body := map[string]any{}
			applyCache(body, req)
			_, key := body["prompt_cache_key"]
			if key != tc.key {
				t.Fatalf("key: %#v", body)
			}
			retention, _ := body["prompt_cache_retention"].(string)
			if retention != tc.retention {
				t.Fatalf("retention: %#v", body)
			}
			options, modern := body["prompt_cache_options"]
			if modern != tc.modern || modern && !reflect.DeepEqual(options, map[string]any{"mode": "implicit", "ttl": "30m"}) {
				t.Fatalf("options: %#v", body)
			}
		})
	}
}

func TestCacheKeyStableAndPrivate(t *testing.T) {
	req := Request{Provider: Provider{BaseURL: "https://api.openai.com/v1", APIKey: "secret"}, Model: "gpt-5.4", System: "cwd: /private/project", Tools: true}
	first := map[string]any{}
	applyCache(first, req)
	req.Provider.APIKey = "rotated"
	req.Msgs = []Msg{{Role: RoleUser, Text: "changing user text"}}
	second := map[string]any{}
	applyCache(second, req)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("unstable: %v %v", first, second)
	}
	key := first["prompt_cache_key"].(string)
	if len(key) > 64 || strings.Contains(key, "private") || strings.Contains(key, "secret") {
		t.Fatal(key)
	}
	req.Tools = false
	second = map[string]any{}
	applyCache(second, req)
	if first["prompt_cache_key"] == second["prompt_cache_key"] {
		t.Fatal("tool prefix ignored")
	}
	req.Cache = &CacheOptions{Key: "caller", Retention: "in_memory"}
	applyCache(second, req)
	if second["prompt_cache_key"] != "caller" || second["prompt_cache_retention"] != "in_memory" {
		t.Fatal(second)
	}
}

func TestCacheWireUsage(t *testing.T) {
	for _, protocol := range []string{ProtoChat, ProtoResponses} {
		for _, model := range []string{"gpt-5.4", "gpt-6.1-sol", "unknown"} {
			t.Run(protocol+"/"+model, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					want := map[string]any{"prompt_cache_key": "caller"}
					if model == "gpt-5.4" {
						want["prompt_cache_retention"] = "24h"
					} else if model == "gpt-6.1-sol" {
						want["prompt_cache_options"] = map[string]any{"ttl": "30m", "mode": "implicit"}
					}
					got := map[string]any{}
					for key, value := range body {
						if strings.HasPrefix(key, "prompt_cache_") {
							got[key] = value
						}
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("cache body: got %#v, want %#v", got, want)
					}
					if protocol == ProtoChat && (body["messages"] == nil || body["input"] != nil) || protocol == ProtoResponses && (body["input"] == nil || body["messages"] != nil) {
						t.Errorf("endpoint body: %#v", body)
					}
					if protocol == ProtoChat {
						_, _ = w.Write([]byte("data: {\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":10,\"prompt_tokens_details\":{\"cached_tokens\":60,\"cache_write_tokens\":30}}}\n\ndata: [DONE]\n\n"))
					} else {
						_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":100,\"output_tokens\":10,\"input_tokens_details\":{\"cached_tokens\":60,\"cache_write_tokens\":30}}}}\n\n"))
					}
				}))
				defer srv.Close()
				var client Client = chatClient{}
				if protocol == ProtoResponses {
					client = responsesClient{}
				}
				var usage Usage
				err := client.Stream(context.Background(), Request{Provider: Provider{BaseURL: srv.URL, Protocol: protocol}, Model: model, System: "unchanged", Msgs: []Msg{{Role: RoleUser, Text: "hello"}}, Cache: &CacheOptions{Key: "caller"}}, func(e Event) {
					if e.Kind == EvDone {
						usage = e.Usage
					}
				})
				if err != nil || usage != (Usage{In: 100, Out: 10, Cached: 60, CacheWrite: 30}) {
					t.Fatalf("usage %+v: %v", usage, err)
				}
			})
		}
	}
}
