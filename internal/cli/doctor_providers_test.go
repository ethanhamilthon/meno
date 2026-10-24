package cli

import (
	"context"
	"testing"

	"meno/internal/llm"
)

func TestDoctorCountsOnlyEnabledAPIProviders(t *testing.T) {
	providers := []llm.Provider{
		{Name: "keyed", BaseURL: "https://example.test/v1", APIKey: "key", Protocol: llm.ProtoResponses},
		{Name: "proxy", BaseURL: "http://127.0.0.1/v1", Run: "exec sleep 60", Protocol: llm.ProtoResponses},
		{Name: "disabled", BaseURL: "https://example.test/v1", APIKey: "key", Disabled: true, Protocol: llm.ProtoResponses},
	}
	_, _, _, a, _ := startRigWith(t, &scripted{}, providers, func(context.Context, llm.Provider) ([]llm.Model, error) { return nil, nil })
	checks := a.daemonChecks(false)
	got, ok := find(DoctorReport{Checks: checks}, "providers")
	if !ok || got.Status != "ok" || got.Detail != "2 usable of 3" {
		t.Fatalf("provider check: %+v", got)
	}
}
