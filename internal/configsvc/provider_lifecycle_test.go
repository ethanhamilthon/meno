package configsvc

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/llm"
)

func TestLegacyHelperDataIsIgnoredAndCustomRunStillWorks(t *testing.T) {
	paths := config.Paths{Home: t.TempDir(), Cwd: t.TempDir()}
	legacy := filepath.Join(paths.Home, "cliproxy")
	if err := os.MkdirAll(filepath.Join(legacy, "auth"), 0700); err != nil {
		t.Fatal(err)
	}
	state := []byte(`{"accounts":2,"accounts_known":true,"has_accounts":true,"port":1900}`)
	statePath := filepath.Join(legacy, "state.json")
	tokenPath := filepath.Join(legacy, "auth", "account.json")
	for path, data := range map[string][]byte{statePath: state, tokenPath: []byte("private-token")} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	st := config.NewStore(paths)
	if err := st.SaveProviders([]llm.Provider{{Name: "custom", BaseURL: "http://127.0.0.1:8317/v1", Protocol: llm.ProtoResponses, Run: "exec sleep 60"}}); err != nil {
		t.Fatal(err)
	}
	svc, err := New(Options{Paths: paths,
		FetchMetadata: func(context.Context) (map[string]llm.ModelInfo, error) { return nil, nil },
		FetchModels:   func(context.Context, llm.Provider) ([]llm.Model, error) { return []llm.Model{{ID: "local-model"}}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ps := svc.Providers()
	if len(ps) != 1 || ps[0].Name != "custom" {
		t.Fatalf("providers: %+v", ps)
	}
	if svc.daemons.Running("oauth") || !svc.daemons.Running("custom") {
		t.Fatal("only the configured run command should start")
	}
	_, cl := startConfigClient(t, paths.Home, cfgBackend{svc: svc})
	for _, route := range []string{"", "/services", "/accounts", "/accounts/account.json", "/helper", "/start", "/status", "/callback", "/restart"} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
			path := "/v1/login" + route
			_, err := svc.Resource(context.Background(), method, path, "", api.ResourceRequest{})
			var apiError *daemon.APIError
			if !errors.As(err, &apiError) || apiError.Code != api.CodeNotFound {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			req, err := http.NewRequest(method, "http://meno"+path, strings.NewReader(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			res, err := cl.HTTP.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != http.StatusNotFound {
				t.Fatalf("%s %s: HTTP %d", method, path, res.StatusCode)
			}
		}
	}
	svc.syncProvider(llm.Provider{Name: "custom", Disabled: true})
	if svc.daemons.Running("custom") {
		t.Fatal("disabled provider process still running")
	}
	svc.syncProvider(ps[0])
	if !svc.daemons.Running("custom") {
		t.Fatal("custom provider did not restart")
	}
	svc.Close()
	if svc.daemons.Running("custom") {
		t.Fatal("provider process survived service close")
	}
	for path, want := range map[string]string{statePath: string(state), tokenPath: "private-token"} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("legacy data changed at %s: %v", path, err)
		}
	}
}
