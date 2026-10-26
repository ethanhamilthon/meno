package configsvc

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/llm"
)

type cfgBackend struct {
	daemon.NotImplementedBackend
	svc *Service
}

func (b cfgBackend) Resource(ctx context.Context, m, p, q string, r api.ResourceRequest) (api.ResourceResponse, error) {
	return b.svc.Resource(ctx, m, p, q, r)
}

func TestDaemonConfigRoutesAndTypedClient(t *testing.T) {
	home, cwd := t.TempDir(), t.TempDir()
	paths := config.Paths{Home: home, Cwd: cwd}
	events := []string{}
	svc, err := New(Options{Paths: paths, Notify: func(e string) { events = append(events, e) }, FetchMetadata: func(context.Context) (map[string]llm.ModelInfo, error) { return map[string]llm.ModelInfo{}, nil }, FetchModels: func(context.Context, llm.Provider) ([]llm.Model, error) { return []llm.Model{{ID: "live"}}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	_, cl := startConfigClient(t, home, cfgBackend{svc: svc})
	settings, err := cl.Settings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if settings.Settings.Effort != "high" {
		t.Fatalf("settings: %+v", settings)
	}
	settings.Settings.Theme = "test-theme"
	saved, err := cl.PutSettings(context.Background(), settings.Settings)
	if err != nil || saved.Settings.Theme != "test-theme" {
		t.Fatalf("settings save: %+v %v", saved, err)
	}
	provider, err := cl.CreateProvider(context.Background(), api.ConfigProvider{Name: "test", BaseURL: "https://example.test/v1", APIKey: "key", Protocol: llm.ProtoChat})
	if err != nil || provider.Provider.Name != "test" {
		t.Fatalf("create: %+v %v", provider, err)
	}
	_, err = cl.SetProviderModels(context.Background(), "test", map[string]api.ConfigModelSettings{"live": {Efforts: []string{"low", "high"}}})
	if err != nil {
		t.Fatal(err)
	}
	models, err := cl.Models(context.Background(), api.ModelsRequest{Refresh: true, RefreshCatalog: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(models.Models) != 1 || models.Models[0].ID != "live" {
		t.Fatalf("models: %+v", models)
	}
	updated, err := cl.SetProviderEnabled(context.Background(), "test", false)
	if err != nil || !updated.Provider.Disabled {
		t.Fatalf("disable: %+v %v", updated, err)
	}
	updated, err = cl.UpdateProvider(context.Background(), "test", api.ConfigProvider{Name: "test", BaseURL: "https://example.test/v1", Protocol: llm.ProtoChat})
	if err != nil || !updated.Provider.HasAPIKey {
		t.Fatalf("preserve key: %+v %v", updated, err)
	}
	if err = cl.DeleteProvider(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	if len(events) < 3 {
		t.Fatalf("notifications: %v", events)
	}
}

func TestProviderAndMCPFilesReloadAndAreEditable(t *testing.T) {
	paths := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	svc, err := New(Options{Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	b := cfgBackend{svc: svc}
	_, err = b.Resource(context.Background(), "GET", "/v1/providers", "", api.ResourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(paths.ProviderFile(), []byte("[{\"name\":\"overlay\",\"base_url\":\"https://example.test/v1\",\"api_key\":\"x\",\"protocol\":\"chat completions\"}]"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := b.Resource(context.Background(), "GET", "/v1/providers", "", api.ResourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var got api.ProvidersResponse
	if err = json.Unmarshal(r.Data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Providers) != 1 || got.Providers[0].Name != "overlay" {
		t.Fatalf("providers: %+v", got)
	}
	if _, err = b.Resource(context.Background(), "DELETE", "/v1/providers/overlay", "", api.ResourceRequest{}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(paths.ProviderFile()); strings.Contains(string(data), "overlay") {
		t.Fatalf("provider still in the file: %s", data)
	}
	if err = os.WriteFile(paths.MCPFile(), []byte("[{\"name\":\"server\",\"transport\":\"stdio\",\"command\":\"true\"}]"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err = b.Resource(context.Background(), "GET", "/v1/mcp/servers", "", api.ResourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(r.Data), "server") {
		t.Fatalf("mcp: %s", r.Data)
	}
}

func TestSettingsFileReloadAndValidation(t *testing.T) {
	paths := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	svc, _ := New(Options{Paths: paths})
	b := cfgBackend{svc: svc}
	_, err := b.Resource(context.Background(), "GET", "/v1/settings", "", api.ResourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(paths.SettingsFile(), []byte("{\"effort\":\"low\",\"theme\":\"outside\"}"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := b.Resource(context.Background(), "GET", "/v1/settings", "", api.ResourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var setting api.SettingsResponse
	if err = json.Unmarshal(r.Data, &setting); err != nil {
		t.Fatal(err)
	}
	if setting.Settings.Effort != "low" || setting.Settings.Theme != "outside" {
		t.Fatalf("reload: %+v", setting)
	}
	if _, err = b.Resource(context.Background(), "PUT", "/v1/settings", "", api.ResourceRequest{Data: json.RawMessage("{\"settings\":{\"view\":7}}")}); err == nil {
		t.Fatal("expected settings payload validation")
	}
}

func startConfigClient(t *testing.T, home string, backend daemon.Backend) (*daemon.Server, *client.Client) {
	t.Helper()
	srv, err := daemon.New(daemon.Options{Home: home, Backend: backend, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { srv.Stop("test") })
	go func() { _ = srv.Serve(ctx) }()
	c := client.New(home)
	c.HTTP = &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", srv.Socket())
	}}}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err = c.Hello(context.Background()); err == nil {
			return srv, c
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("daemon did not start")
	return nil, nil
}

func TestFileSearchAndDebug(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "find-me.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	svc, _ := New(Options{Paths: config.Paths{Home: t.TempDir(), Cwd: cwd}, DebugRequests: func(id string) []json.RawMessage {
		return []json.RawMessage{json.RawMessage("{\"session\":\"" + id + "\"}")}
	}})
	resp, err := svc.Resource(context.Background(), "GET", "/v1/fs/search", url.Values{"root": {cwd}, "q": {"find-me"}}.Encode(), api.ResourceRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resp.Data), "find-me.txt") {
		t.Fatalf("search %s", resp.Data)
	}
	resp, err = svc.Resource(context.Background(), "GET", "/v1/debug/requests", "session=s", api.ResourceRequest{})
	if err != nil || !strings.Contains(string(resp.Data), `"session":"s"`) {
		t.Fatalf("debug %s %v", resp.Data, err)
	}
}

// A key written as env:NAME is shown to the engine with the variable's value
// and stays a reference on disk, even after the UI changes the provider.
func TestEnvKeyStaysAReferenceOnDisk(t *testing.T) {
	t.Setenv("MENO_TEST_KEY", "from-env")
	paths := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	if err := os.WriteFile(paths.ProviderFile(), []byte(`[{"name":"p","base_url":"https://x/v1","api_key":"env:MENO_TEST_KEY","protocol":"chat completions"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	svc, err := New(Options{Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if got := svc.Providers(); len(got) != 1 || got[0].APIKey != "from-env" {
		t.Fatalf("engine list: %+v", got)
	}
	body := json.RawMessage(`{"provider":{"name":"p","base_url":"https://x/v1","protocol":"chat completions","disabled":true}}`)
	if _, err = (cfgBackend{svc: svc}).Resource(context.Background(), "PUT", "/v1/providers/p", "", api.ResourceRequest{Data: body}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(paths.ProviderFile())
	if !strings.Contains(string(data), `"env:MENO_TEST_KEY"`) || strings.Contains(string(data), "from-env") || !strings.Contains(string(data), `"disabled": true`) {
		t.Fatalf("file after the edit:\n%s", data)
	}
}

// A hand edit of mcp.json is seen by the next read without a restart.
func TestHandEditOfMCPFileIsSeen(t *testing.T) {
	paths := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	svc, err := New(Options{Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	b := cfgBackend{svc: svc}
	read := func() string {
		r, err := b.Resource(context.Background(), "GET", "/v1/mcp/servers", "", api.ResourceRequest{})
		if err != nil {
			t.Fatal(err)
		}
		return string(r.Data)
	}
	if strings.Contains(read(), "hand") {
		t.Fatal("unexpected server")
	}
	if err = os.WriteFile(paths.MCPFile(), []byte(`[{"name":"hand","command":"true"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(), "hand") {
		t.Fatal("the edit was not seen")
	}
}
