package configsvc

import (
	"context"
	"encoding/json"
	"meno/internal/api"
	"meno/internal/config"
	"reflect"
	"strings"
	"testing"
)

func TestConfigBundleMatchesIndividualReadsAndRedactsKeys(t *testing.T) {
	paths := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	svc, err := New(Options{Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	_, cl := startConfigClient(t, paths.Home, cfgBackend{svc: svc})
	ctx := context.Background()
	_, err = cl.CreateProvider(ctx, api.ConfigProvider{Name: "test", BaseURL: "https://example.test/v1", APIKey: "private-key", Protocol: "chat completions"})
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := cl.Config(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := cl.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	providers, err := cl.Providers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	servers, err := cl.McpServers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	skills, err := cl.Skills(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bundle, api.ConfigBundle{Settings: settings, Providers: providers, MCP: servers, Skills: skills}) {
		t.Fatalf("bundle differs %+v", bundle)
	}
	raw, _ := json.Marshal(bundle)
	if strings.Contains(string(raw), "private-key") || !bundle.Providers.Providers[0].HasAPIKey {
		t.Fatalf("key redaction %s", raw)
	}
	if _, err = svc.bundleResource("POST"); err == nil {
		t.Fatal("bundle is read-only")
	}
}
