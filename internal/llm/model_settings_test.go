package llm

import (
	"context"
	"net"
	"strings"
	"testing"
)

func f(v float64) *float64 { return &v }

func TestModelSettingsWinOverTheCatalogFieldByField(t *testing.T) {
	catalog := map[string]ModelInfo{"openai/gpt-x": {Context: 1000, Input: 1, Output: 2}}
	p := Provider{Name: "openai", BaseURL: "https://api.openai.com/v1", Models: map[string]ModelSettings{
		"gpt-x": {Context: 5000, Output: f(9), CacheRead: f(0.1)},
	}}
	info, ok := p.Lookup(catalog, "gpt-x")
	if !ok || info.Context != 5000 || info.Input != 1 || info.Output != 9 || info.CacheRead == nil || *info.CacheRead != 0.1 || info.MissingPricing {
		t.Fatalf("%+v %v", info, ok)
	}
}

func TestModelSettingsForAnUnknownModelNeedBothPricesToPrice(t *testing.T) {
	p := Provider{Name: "local", BaseURL: "http://127.0.0.1:8317/v1", Models: map[string]ModelSettings{
		"mine":  {Context: 32000, Input: f(1)},
		"mine2": {Context: 32000, Input: f(1), Output: f(3)},
	}}
	info, ok := p.Lookup(nil, "mine")
	if !ok || info.Context != 32000 || !info.MissingPricing || info.CanPrice(Usage{In: 1}) {
		t.Fatalf("half a price must not price: %+v %v", info, ok)
	}
	info, _ = p.Lookup(nil, "mine2")
	if info.MissingPricing || info.Cost(Usage{In: 1_000_000, Out: 1_000_000}) != 4 {
		t.Fatalf("%+v", info)
	}
	if _, ok := p.Lookup(nil, "other"); ok {
		t.Fatal("a model without settings or catalog entry stays unknown")
	}
}

func TestEffortsAndClamp(t *testing.T) {
	std := []string{"low", "medium", "high", "max"}
	p := Provider{Models: map[string]ModelSettings{"m": {Efforts: []string{"fast", "slow"}}}}
	if got := p.EffortsFor("m", std); strings.Join(got, ",") != "fast,slow" {
		t.Fatal(got)
	}
	if got := p.EffortsFor("other", std); len(got) != 4 {
		t.Fatal(got)
	}
	for _, c := range []struct {
		allowed []string
		cur     string
		want    string
	}{
		{std, "max", "max"},
		{[]string{"low", "medium"}, "max", "medium"},
		{[]string{"low", "high"}, "max", "high"},
		{[]string{"fast", "slow"}, "high", "fast"},
		{nil, "high", "high"},
	} {
		if got := ClampEffort(c.allowed, c.cur); got != c.want {
			t.Errorf("ClampEffort(%v, %q) = %q, want %q", c.allowed, c.cur, got, c.want)
		}
	}
}

func TestValidateModels(t *testing.T) {
	good := map[string]ModelSettings{"a": {Context: 1, Input: f(0), Efforts: []string{"x", "y"}}}
	if err := ValidateModels(good); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]map[string]ModelSettings{
		"empty id":  {" ": {}},
		"context":   {"a": {Context: -1}},
		"price":     {"a": {Output: f(-1)}},
		"duplicate": {"a": {Efforts: []string{"x", "x"}}},
		"space":     {"a": {Efforts: []string{"very high"}}},
	} {
		if err := ValidateModels(bad); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

// A proxy that started a moment ago refuses connections: that must be retried.
func TestConnectionRefusedIsRetryable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	_, err = post(context.Background(), Provider{BaseURL: "http://" + addr}, "/x", map[string]any{}, nil)
	if err == nil {
		t.Fatal("the port is closed, the request must fail")
	}
	if ok, _ := Retryable(err); !ok {
		t.Fatalf("connection refused must be retryable: %v", err)
	}
}
