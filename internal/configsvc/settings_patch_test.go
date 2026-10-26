package configsvc

import (
	"context"
	"testing"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/llm"
)

func patchFixture(t *testing.T) *Service {
	t.Helper()
	svc, err := New(Options{Paths: config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}, Notify: func(string) {}, FetchMetadata: func(context.Context) (map[string]llm.ModelInfo, error) { return nil, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	return svc
}

func patch(t *testing.T, svc *Service, body string) (any, error) {
	t.Helper()
	return svc.settingsResource("PATCH", api.ResourceRequest{Data: []byte(body)})
}

// Two clients edit different fields from the same starting point: both survive.
func TestSettingsPatchMergesDisjointFields(t *testing.T) {
	svc := patchFixture(t)
	if _, err := patch(t, svc, `{"fields":{"system_prompt":"dark"}}`); err != nil {
		t.Fatal(err)
	}
	_, err := patch(t, svc, `{"fields":{"notify_volume":60}}`)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := svc.Settings()
	if got.SystemPrompt != "dark" || got.NotifyVolume != 60 {
		t.Fatalf("theme=%q volume=%d", got.SystemPrompt, got.NotifyVolume)
	}
}

// A stale expectation on the same field is a conflict, not a silent overwrite.
func TestSettingsPatchRejectsStaleField(t *testing.T) {
	svc := patchFixture(t)
	if _, err := patch(t, svc, `{"fields":{"notify_volume":40}}`); err != nil {
		t.Fatal(err)
	}
	if _, err := patch(t, svc, `{"fields":{"notify_volume":60},"expect":{"notify_volume":80}}`); err == nil {
		t.Fatal("stale edit must conflict")
	}
	if got, _ := svc.Settings(); got.NotifyVolume != 40 {
		t.Fatalf("volume=%d", got.NotifyVolume)
	}
}
