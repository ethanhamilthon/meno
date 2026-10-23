package cli

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"meno/internal/llm"
)

func modelsRig(t *testing.T, calls *atomic.Int32) *DaemonAPI {
	t.Helper()
	providers := []llm.Provider{
		{Name: "open", BaseURL: "http://127.0.0.1", Protocol: llm.ProtoResponses},
		{Name: "broken", BaseURL: "http://127.0.0.1", Protocol: llm.ProtoResponses},
	}
	fetch := func(_ context.Context, p llm.Provider) ([]llm.Model, error) {
		calls.Add(1)
		if p.Name == "broken" {
			return nil, errors.New("models: http 401: no key")
		}
		return []llm.Model{{Provider: p.Name, ID: "alpha-1"}, {Provider: p.Name, ID: "beta-2"}}, nil
	}
	_, _, _, dapi, _ := startRigWith(t, &scripted{}, providers, fetch)
	return dapi
}

func TestModelsListsDiscoveredModelsOfProviderWithoutModelSettings(t *testing.T) {
	var calls atomic.Int32
	dapi := modelsRig(t, &calls)
	code, out, errs := runE2E(t, dapi, "models", "--provider", "open")
	if code != 0 {
		t.Fatalf("models: %d %s %s", code, out, errs)
	}
	if out != "open/alpha-1\nopen/beta-2\n" {
		t.Fatalf("models output %q", out)
	}
	if errs != "" {
		t.Fatalf("unexpected stderr %q", errs)
	}
}

func TestModelsReportsProviderErrorsOnStderr(t *testing.T) {
	var calls atomic.Int32
	dapi := modelsRig(t, &calls)
	code, out, errs := runE2E(t, dapi, "models", "--refresh")
	if code != 0 {
		t.Fatalf("models: %d %s %s", code, out, errs)
	}
	if !strings.Contains(out, "open/alpha-1") || strings.Contains(out, "broken/") {
		t.Fatalf("models output %q", out)
	}
	if !strings.Contains(errs, "meno: models: broken: models: http 401: no key") {
		t.Fatalf("stderr %q", errs)
	}
}

func TestModelsRefreshAsksProvidersAgain(t *testing.T) {
	var calls atomic.Int32
	dapi := modelsRig(t, &calls)
	_, out, _ := runE2E(t, dapi, "models", "--provider", "open")
	if !strings.Contains(out, "alpha-1") {
		t.Fatalf("first list %q", out)
	}
	base := calls.Load()
	runE2E(t, dapi, "models", "--provider", "open")
	if calls.Load() != base {
		t.Fatalf("a filled cache must not be asked again: %d -> %d", base, calls.Load())
	}
	runE2E(t, dapi, "models", "--provider", "open", "--refresh")
	if calls.Load() != base+1 {
		t.Fatalf("--refresh must ask the provider once: %d -> %d", base, calls.Load())
	}
}

func TestModelsJSONStaysAList(t *testing.T) {
	var calls atomic.Int32
	dapi := modelsRig(t, &calls)
	_, out, _ := runE2E(t, dapi, "models", "--provider", "open", "--json")
	if !strings.HasPrefix(out, `[{"provider":"open","id":"alpha-1"`) {
		t.Fatalf("json %q", out)
	}
}
